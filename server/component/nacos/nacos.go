// Package nacos 请求健康检查和登录接口，并写入 FG_COMPONENT_NACOS_METRIC。
package nacos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/component/netx"
	"fluxguard/server/component/pane"
	"fluxguard/server/component/row"
	"fluxguard/server/component/runtime"
	"fluxguard/server/component/series"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	runtime.Register(catalog.Nacos, probe{})
}

type probe struct{}

// Sample 是一轮 Nacos 采集。
type Sample struct {
	ConfigCount  float64
	ServiceCount float64
}

func (probe) Probe(ctx context.Context, db *dbx.DB, target conn.Open) (models.Status, error) {
	sample, status := collect(ctx, target)
	if db != nil {
		if err := save(ctx, db, target.TenantID, target.ComponentID, sample, status); err != nil {
			return status, err
		}
	}
	return status, nil
}

func (probe) Series(ctx context.Context, db *dbx.DB, tenantID, componentID string, limit int) ([]models.Point, error) {
	return series.Read(ctx, db, "FG_COMPONENT_NACOS_METRIC", tenantID, componentID, catalog.Columns(catalog.Nacos), limit, series.Span{})
}

func collect(ctx context.Context, target conn.Open) (Sample, models.Status) {
	base, err := netx.BaseURL(target.Address, 8848)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	client := &http.Client{Timeout: 5 * time.Second}
	token := ""
	if target.UserName != "" || target.Secret != "" {
		token, err = login(ctx, client, base, target.UserName, target.Secret)
		if err != nil {
			return Sample{}, models.Status{Error: err.Error()}
		}
	}
	healthURL := base + "/nacos/v1/console/health/readiness"
	if token != "" {
		healthURL += "?accessToken=" + url.QueryEscape(token)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	resp, err := client.Do(req)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 || !healthy(resp.StatusCode, body) {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return Sample{}, models.Status{Error: row.Cut(msg, 100)}
	}
	sample := Sample{}
	if prom := fetch(ctx, client, base+"/nacos/actuator/prometheus", token); prom != "" {
		sample.ConfigCount, sample.ServiceCount = ParseProm(prom)
	}
	return sample, models.Status{
		Up: true, Role: "server",
		Summary: fmt.Sprintf("配置 %.0f · 服务 %.0f", sample.ConfigCount, sample.ServiceCount),
	}
}

func login(ctx context.Context, client *http.Client, base, user, secret string) (string, error) {
	form := url.Values{}
	form.Set("username", user)
	form.Set("password", secret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/nacos/v1/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var payload struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&payload); err != nil || payload.AccessToken == "" {
		return "", fmt.Errorf("Nacos 登录失败")
	}
	return payload.AccessToken, nil
}

func fetch(ctx context.Context, client *http.Client, rawURL, token string) string {
	if token != "" {
		rawURL += "?accessToken=" + url.QueryEscape(token)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ""
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	return string(body)
}

func healthy(status int, body []byte) bool {
	if status < 200 || status >= 300 {
		return false
	}
	text := strings.TrimSpace(string(body))
	if text == "" || strings.EqualFold(text, "OK") || strings.Contains(strings.ToUpper(text), "UP") {
		return true
	}
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		if v, ok := payload["status"].(string); ok {
			return strings.EqualFold(v, "UP")
		}
	}
	return true
}

// ParseProm 从 Prometheus 文本里取配置数和服务数。没有对应行时为 0。
func ParseProm(text string) (configCount, serviceCount float64) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.Contains(line, `name="configCount"`), strings.Contains(line, "nacos_config_count"):
			configCount = lastFloat(line)
		case strings.Contains(line, `name="serviceCount"`), strings.Contains(line, "nacos_service_count"):
			serviceCount = lastFloat(line)
		}
	}
	return configCount, serviceCount
}

func lastFloat(line string) float64 {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.ParseFloat(fields[len(fields)-1], 64)
	return n
}

func save(ctx context.Context, db *dbx.DB, tenantID, componentID string, sample Sample, status models.Status) error {
	args := dbx.Args{
		"tenantId": tenantID, "pointId": id.New(), "componentId": componentID,
		"upFlag": row.Flag(status.Up), "versionText": row.Cut(status.Version, 64),
		"roleName": row.Cut(status.Role, 32), "errorText": row.Cut(status.Error, 100),
		"summaryText": row.Cut(status.Summary, 100), "observedTime": store.FormatTime(time.Now().UTC()),
		"configCount": sample.ConfigCount, "serviceCount": sample.ServiceCount,
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_COMPONENT_NACOS_METRIC (
		  tenantId, pointId, componentId, upFlag, versionText, roleName, errorText, summaryText, observedTime,
		  configCount, serviceCount,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @pointId, @componentId, @upFlag, @versionText, @roleName, @errorText, @summaryText, @observedTime,
		  @configCount, @serviceCount,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	if err != nil {
		return err
	}
	return pane.Touch(ctx, db, catalog.Nacos, args)
}
