package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"fluxguard/common/id"
	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/component/pane"
	"fluxguard/server/component/row"
	"fluxguard/server/component/runtime"
	"fluxguard/server/component/series"
	"fluxguard/server/plugin"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// runner 用插件交回的结果替换一种组件的内置探针。进程不在这里管理。
type runner struct {
	id        string
	component string
}

var (
	bindMu sync.Mutex
	bound  = map[string]runtime.Runner{}
	ident  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
)

// Attach 按已加载插件的 component 字段换掉对应种类的内置探针。
// 种类不在目录里时不替换，并返回错误。插件进程仍由 plugin 保持。
func Attach() error {
	slots := plugin.Slots()
	bindMu.Lock()
	defer bindMu.Unlock()
	for _, slot := range slots {
		if slot.Component == "" {
			continue
		}
		if !catalog.Known(slot.Component) {
			return fmt.Errorf("插件服务的组件种类不在目录里: %s", slot.Component)
		}
		if _, ok := bound[slot.Component]; ok {
			return fmt.Errorf("这种组件已经有插件")
		}
	}
	for _, slot := range slots {
		if slot.Component == "" {
			continue
		}
		prev := runtime.Replace(slot.Component, runner{id: slot.ID, component: slot.Component})
		bound[slot.Component] = prev
	}
	return nil
}

// Detach 把被换掉的内置探针交还。不停止插件进程。
func Detach() {
	bindMu.Lock()
	defer bindMu.Unlock()
	for kind, prev := range bound {
		runtime.Replace(kind, prev)
	}
	bound = map[string]runtime.Runner{}
}

func (r runner) Probe(ctx context.Context, db *dbx.DB, target conn.Open) (models.Status, error) {
	target.KindCode = r.component
	var res ProbeResult
	err := plugin.Call(ctx, r.id, methodProbe, probeParams(target), target.Secret, &res)
	status := models.Status{}
	var values map[string]float64
	var detail string
	if err != nil {
		status.Error = err.Error()
	} else {
		status = models.Status{Up: res.Up, Version: res.Version, Role: res.Role, Error: res.Error, Summary: res.Summary}
		values, err = valuesFor(r.component, res.Up, res.Values)
		if err != nil {
			status = models.Status{Error: err.Error()}
			values = nil
		} else if detail, err = detailText(res.Detail); err != nil {
			status = models.Status{Error: err.Error()}
			values = nil
		}
	}
	if values == nil {
		values, _ = valuesFor(r.component, false, nil)
	}
	if db != nil {
		if saveErr := insertPoint(ctx, db, r.component, target, status, values, detail); saveErr != nil {
			return status, saveErr
		}
	}
	return status, nil
}

func (r runner) Series(ctx context.Context, db *dbx.DB, tenantID, componentID string, limit int) ([]models.Point, error) {
	spec, ok := catalog.Get(r.component)
	if !ok {
		return nil, fmt.Errorf("不支持的组件种类")
	}
	return series.Read(ctx, db, spec.MetricTable, tenantID, componentID, catalog.Columns(r.component), limit, series.Span{})
}

func (r runner) Exec(ctx context.Context, target conn.Open, command string, args map[string]string) (string, error) {
	if err := checkOp(r.id, command, args); err != nil {
		return "", err
	}
	target.KindCode = r.component
	var res MaintainResult
	if err := plugin.Call(ctx, r.id, methodMaintain, maintainParams(target, command, args), target.Secret, &res); err != nil {
		return "", err
	}
	if res.Error != "" {
		return "", fmt.Errorf("%s", res.Error)
	}
	output := res.Output
	if len(output) > maxOutput {
		output = output[:maxOutput]
	}
	return output, nil
}

func checkOp(id, name string, args map[string]string) error {
	allowed, known := plugin.MaintainArgs(id, name)
	if !known {
		return fmt.Errorf("这种组件没有这项操作")
	}
	if len(args) > maxArgs {
		return fmt.Errorf("操作参数过多")
	}
	for key, arg := range args {
		if len(arg) > maxArgLen {
			return fmt.Errorf("操作参数过长")
		}
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("这项操作没有这个参数")
		}
	}
	return nil
}

func probeParams(target conn.Open) ProbeParams {
	return ProbeParams{
		Kind: target.KindCode, Address: target.Address, User: target.UserName,
		Secret: target.Secret, Database: target.DatabaseName, Option: target.OptionText,
	}
}

func maintainParams(target conn.Open, op string, args map[string]string) MaintainParams {
	return MaintainParams{
		Op: op, Address: target.Address, User: target.UserName, Secret: target.Secret,
		Database: target.DatabaseName, Option: target.OptionText, Args: args,
	}
}

func valuesFor(kind string, up bool, in map[string]float64) (map[string]float64, error) {
	cols := catalog.Columns(kind)
	allowed := make(map[string]struct{}, len(cols))
	for _, col := range cols {
		allowed[col] = struct{}{}
	}
	out := make(map[string]float64, len(cols))
	for key, value := range in {
		if _, ok := allowed[key]; !ok {
			return nil, fmt.Errorf("插件返回了目录外的指标")
		}
		out[key] = value
	}
	for _, col := range cols {
		if _, ok := out[col]; ok {
			continue
		}
		if up {
			return nil, fmt.Errorf("插件缺少指标 %s", col)
		}
		out[col] = -1
	}
	return out, nil
}

func detailText(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", nil
	}
	if len(raw) > maxDetail || !json.Valid(raw) {
		return "", fmt.Errorf("插件应答无效")
	}
	return string(raw), nil
}

func insertPoint(ctx context.Context, db *dbx.DB, kind string, target conn.Open, status models.Status, values map[string]float64, detail string) error {
	spec, ok := catalog.Get(kind)
	if !ok || !ident.MatchString(spec.MetricTable) {
		return fmt.Errorf("采集表无效")
	}
	cols := catalog.Columns(kind)
	args := dbx.Args{
		"tenantId": target.TenantID, "pointId": id.New(), "componentId": target.ComponentID,
		"upFlag": row.Flag(status.Up), "versionText": row.Cut(status.Version, 64),
		"roleName": row.Cut(status.Role, 32), "errorText": row.Cut(status.Error, 100),
		"summaryText": row.Cut(status.Summary, 100), "observedTime": store.FormatTime(time.Now().UTC()),
		"extProperty": detail,
	}
	for _, col := range cols {
		if !ident.MatchString(col) {
			return fmt.Errorf("指标列无效")
		}
		args[col] = values[col]
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	names := append([]string{
		"tenantId", "pointId", "componentId", "upFlag", "versionText", "roleName", "errorText", "summaryText", "observedTime",
	}, cols...)
	names = append(names, "extProperty", "addTime", "addWho", "editTime", "editWho", "oprSeqFlag", "currentVersion", "activeFlag", "noteText")
	holders := make([]string, len(names))
	for i, name := range names {
		holders[i] = "@" + name
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", spec.MetricTable, strings.Join(names, ", "), strings.Join(holders, ", "))
	if _, err := db.Exec(ctx, query, args); err != nil {
		return err
	}
	return pane.Touch(ctx, db, kind, args)
}

var (
	_ runtime.Runner    = runner{}
	_ runtime.Commander = runner{}
)
