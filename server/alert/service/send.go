package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fluxguard/server/alert/channel"
	"fluxguard/server/alert/models"
	"fluxguard/server/alert/template"
	"fluxguard/server/alert/validate"
	"fluxguard/server/limits"
	"fluxguard/server/store"
)

// DingSign 计算钉钉机器人加签。sign 是未转义的 Base64，拼到 URL 时再编码。
func DingSign(secret string, milli int64) (timestamp, sign string) {
	ts := strconv.FormatInt(milli, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ts + "\n" + secret))
	return ts, base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func feishuSign(secret string, sec int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", sec, secret)
	mac := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// TestChannel 用样例数据写一条预警日志并立即发送。
func (s *Service) TestChannel(tenantID, channelID string) (models.Log, error) {
	ctx := context.Background()
	view, _, err := s.channels.Load(ctx, tenantID, strings.TrimSpace(channelID))
	if err != nil {
		if s.channels.Missing(err) {
			return models.Log{}, validate.ErrText("渠道不存在")
		}
		return models.Log{}, err
	}
	if view.StatusFlag != "Y" || view.ActiveFlag != "Y" {
		return models.Log{}, validate.ErrText("渠道已停用")
	}
	extra, err := json.Marshal(template.SampleVars())
	if err != nil {
		return models.Log{}, err
	}
	item, err := s.events.Insert(ctx, tenantID, models.Notice{
		Level: "critical", Type: "test", Scene: "fire", Title: "渠道测试",
		Content: "这是一条测试通知", ChannelID: view.ChannelID, Extra: string(extra),
	}, time.Now().UTC())
	if err != nil {
		return models.Log{}, err
	}
	s.sendLog(ctx, tenantID, item)
	return s.events.Get(ctx, tenantID, item.AlertLogID)
}

func (s *Service) sendLog(ctx context.Context, tenantID string, item models.Log) {
	if item.SendStatus == "ok" || item.FailCount >= 3 {
		return
	}
	sentAt := store.FormatTime(validate.StoreTime())
	view, cfg, err := s.channels.Load(ctx, tenantID, strings.TrimSpace(item.ChannelID))
	if err != nil || view.StatusFlag != "Y" || view.ActiveFlag != "Y" {
		text := "渠道不可用"
		if err != nil {
			text = err.Error()
		}
		_ = s.events.MarkSent(ctx, tenantID, item.AlertLogID, "fail", sentAt, "", validate.Clip(text, 500), item.FailCount+1)
		return
	}
	tpl := s.tpl.ForChannel(ctx, tenantID, view, item.SceneCode)
	vars := template.MessageVars(item)
	subject := template.Render(tpl.SubjectText, vars)
	body := template.Render(tpl.BodyText, vars)
	target, err := deliver(ctx, view.ChannelKind, cfg, tpl.ContentType, subject, body, nil)
	status := "ok"
	errText := ""
	fails := item.FailCount
	if err != nil {
		status = "fail"
		errText = err.Error()
		fails++
	}
	result, _ := json.Marshal(map[string]string{
		"target": target, "subject": subject, "body": body, "channelKind": view.ChannelKind,
	})
	_ = s.events.MarkSent(ctx, tenantID, item.AlertLogID, status, sentAt, string(result), validate.Clip(errText, 500), fails)
}

func deliver(ctx context.Context, kind string, cfg map[string]any, contentType, subject, body string, contacts []models.Contact) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, limits.AlertDeliver)
	defer cancel()
	switch kind {
	case "email":
		return sendEmail(ctx, cfg, contentType, subject, body, contacts)
	case "wecom":
		return sendWecom(ctx, cfg, contentType, body)
	case "dingtalk":
		return sendDing(ctx, cfg, subject, body)
	case "feishu":
		return sendFeishu(ctx, cfg, subject, body)
	case "qq":
		return sendQQ(ctx, cfg, body, contacts)
	case "webhook":
		return sendWebhook(ctx, cfg, contentType, subject, body)
	case "sms":
		return sendSMS(ctx, cfg, body, contacts)
	default:
		return "", validate.ErrText("渠道种类无效")
	}
}

func sendEmail(ctx context.Context, cfg map[string]any, contentType, subject, body string, contacts []models.Contact) (string, error) {
	host := strings.TrimSpace(validate.AsText(cfg["host"]))
	port := validate.AsInt(cfg["port"])
	if port <= 0 {
		port = 465
	}
	from := strings.TrimSpace(validate.AsText(cfg["from"]))
	if from == "" {
		from = strings.TrimSpace(validate.AsText(cfg["username"]))
	}
	if host == "" || from == "" {
		return "", validate.ErrText("邮件渠道缺少主机或发件人")
	}
	var tos []string
	tos = append(tos, validate.SplitList(validate.AsText(cfg["to"]))...)
	for _, contact := range contacts {
		if contact.EmailAddr != "" {
			tos = append(tos, contact.EmailAddr)
		}
	}
	tos = validate.SplitList(strings.Join(tos, ","))
	if len(tos) == 0 {
		return "", validate.ErrText("邮件没有收件人")
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{}
	if deadline, ok := ctx.Deadline(); ok {
		dialer.Deadline = deadline
	} else {
		dialer.Timeout = limits.AlertDeliver
	}
	var conn net.Conn
	var err error
	if port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: host})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return strings.Join(tos, ","), err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return strings.Join(tos, ","), err
	}
	defer client.Close()
	if port != 465 && (validate.Flag(validate.AsText(cfg["tls"]), "N") == "Y" || port == 587) {
		if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return strings.Join(tos, ","), err
		}
	}
	username := strings.TrimSpace(validate.AsText(cfg["username"]))
	if username != "" {
		if err := client.Auth(smtp.PlainAuth("", username, channel.PlainSecret(cfg, "password"), host)); err != nil {
			return strings.Join(tos, ","), err
		}
	}
	if err := client.Mail(from); err != nil {
		return strings.Join(tos, ","), err
	}
	for _, to := range tos {
		if err := client.Rcpt(to); err != nil {
			return strings.Join(tos, ","), err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return strings.Join(tos, ","), err
	}
	kind := "text/plain"
	if contentType == "html" {
		kind = "text/html"
	}
	msg := "From: " + from + "\r\nTo: " + strings.Join(tos, ",") + "\r\nSubject: =?UTF-8?B?" +
		base64.StdEncoding.EncodeToString([]byte(subject)) + "?=\r\nMIME-Version: 1.0\r\nContent-Type: " +
		kind + "; charset=UTF-8\r\n\r\n" + body
	if _, err := writer.Write([]byte(msg)); err != nil {
		_ = writer.Close()
		return strings.Join(tos, ","), err
	}
	if err := writer.Close(); err != nil {
		return strings.Join(tos, ","), err
	}
	return strings.Join(tos, ","), client.Quit()
}

func sendWecom(ctx context.Context, cfg map[string]any, contentType, body string) (string, error) {
	hook := strings.TrimSpace(validate.AsText(cfg["webhook"]))
	if hook == "" {
		return "", validate.ErrText("缺少企业微信 Webhook")
	}
	var payload any
	if contentType == "markdown" {
		payload = map[string]any{"msgtype": "markdown", "markdown": map[string]string{"content": body}}
	} else {
		payload = map[string]any{"msgtype": "text", "text": map[string]string{"content": body}}
	}
	return hook, postJSON(ctx, hook, payload, nil)
}

func sendDing(ctx context.Context, cfg map[string]any, subject, body string) (string, error) {
	hook := strings.TrimSpace(validate.AsText(cfg["webhook"]))
	if hook == "" {
		return "", validate.ErrText("缺少钉钉 Webhook")
	}
	if secret := channel.PlainSecret(cfg, "secret"); secret != "" {
		ts, sign := DingSign(secret, time.Now().UnixMilli())
		parsed, err := url.Parse(hook)
		if err != nil {
			return hook, err
		}
		query := parsed.Query()
		query.Set("timestamp", ts)
		query.Set("sign", sign)
		parsed.RawQuery = query.Encode()
		hook = parsed.String()
	}
	title := subject
	if title == "" {
		title = "预警"
	}
	payload := map[string]any{"msgtype": "markdown", "markdown": map[string]string{"title": title, "text": body}}
	return hook, postJSON(ctx, hook, payload, nil)
}

func sendFeishu(ctx context.Context, cfg map[string]any, subject, body string) (string, error) {
	hook := strings.TrimSpace(validate.AsText(cfg["webhook"]))
	if hook == "" {
		return "", validate.ErrText("缺少飞书 Webhook")
	}
	payload := map[string]any{"msg_type": "text", "content": map[string]string{"text": body}}
	if secret := channel.PlainSecret(cfg, "secret"); secret != "" {
		sec := time.Now().Unix()
		payload["timestamp"] = strconv.FormatInt(sec, 10)
		payload["sign"] = feishuSign(secret, sec)
	}
	if subject != "" {
		payload = map[string]any{
			"msg_type": "post",
			"content": map[string]any{
				"post": map[string]any{
					"zh_cn": map[string]any{
						"title":   subject,
						"content": []any{[]any{map[string]string{"tag": "text", "text": body}}},
					},
				},
			},
		}
		if secret := channel.PlainSecret(cfg, "secret"); secret != "" {
			sec := time.Now().Unix()
			payload["timestamp"] = strconv.FormatInt(sec, 10)
			payload["sign"] = feishuSign(secret, sec)
		}
	}
	return hook, postJSON(ctx, hook, payload, nil)
}

func sendQQ(ctx context.Context, cfg map[string]any, body string, contacts []models.Contact) (string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(validate.AsText(cfg["endpoint"])), "/")
	if endpoint == "" {
		return "", validate.ErrText("缺少 QQ 机器人地址")
	}
	if !strings.Contains(endpoint, "send_msg") {
		endpoint += "/send_msg"
	}
	targetType := strings.TrimSpace(validate.AsText(cfg["targetType"]))
	if targetType == "" {
		targetType = "group"
	}
	targets := validate.SplitList(validate.AsText(cfg["targetId"]))
	if targetType == "private" && len(targets) == 0 {
		for _, contact := range contacts {
			if contact.QQAccount != "" {
				targets = append(targets, contact.QQAccount)
			}
		}
	}
	if len(targets) == 0 {
		return "", validate.ErrText("QQ 没有发送目标")
	}
	headers := map[string]string{}
	if token := channel.PlainSecret(cfg, "accessToken"); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	var failed []string
	for _, target := range targets {
		payload := map[string]any{"message": body, "message_type": targetType}
		if targetType == "private" {
			payload["user_id"] = jsonID(target)
		} else {
			payload["group_id"] = jsonID(target)
		}
		if err := postJSON(ctx, endpoint, payload, headers); err != nil {
			failed = append(failed, target+": "+err.Error())
		}
	}
	joined := strings.Join(targets, ",")
	if len(failed) > 0 {
		return joined, validate.ErrText(strings.Join(failed, "; "))
	}
	return joined, nil
}

func sendWebhook(ctx context.Context, cfg map[string]any, contentType, subject, body string) (string, error) {
	rawURL := strings.TrimSpace(validate.AsText(cfg["url"]))
	if rawURL == "" {
		return "", validate.ErrText("缺少 Webhook 地址")
	}
	headers := headerMap(cfg)
	if token := channel.PlainSecret(cfg, "token"); token != "" {
		if headers == nil {
			headers = map[string]string{}
		}
		if _, ok := headers["Authorization"]; !ok {
			headers["Authorization"] = "Bearer " + token
		}
	}
	if strings.TrimSpace(validate.AsText(cfg["bodyMode"])) == "raw" {
		kind := "text/plain; charset=utf-8"
		if contentType == "html" {
			kind = "text/html; charset=utf-8"
		}
		if contentType == "markdown" {
			kind = "text/markdown; charset=utf-8"
		}
		return rawURL, postRaw(ctx, rawURL, kind, []byte(body), headers)
	}
	payload := map[string]any{"subject": subject, "body": body, "contentType": contentType}
	return rawURL, postJSON(ctx, rawURL, payload, headers)
}

func sendSMS(ctx context.Context, cfg map[string]any, body string, contacts []models.Contact) (string, error) {
	rawURL := strings.TrimSpace(validate.AsText(cfg["url"]))
	if rawURL == "" {
		return "", validate.ErrText("缺少短信网关地址")
	}
	mobiles := validate.SplitList(validate.AsText(cfg["to"]))
	for _, contact := range contacts {
		if contact.MobileNo != "" {
			mobiles = append(mobiles, contact.MobileNo)
		}
	}
	mobiles = validate.SplitList(strings.Join(mobiles, ","))
	if len(mobiles) == 0 {
		return "", validate.ErrText("短信没有手机号")
	}
	headers := headerMap(cfg)
	if token := channel.PlainSecret(cfg, "token"); token != "" {
		if headers == nil {
			headers = map[string]string{}
		}
		if _, ok := headers["Authorization"]; !ok {
			headers["Authorization"] = "Bearer " + token
		}
	}
	joined := strings.Join(mobiles, ",")
	if strings.TrimSpace(validate.AsText(cfg["bodyMode"])) == "raw" {
		return joined, postRaw(ctx, rawURL, "text/plain; charset=utf-8", []byte(body), headers)
	}
	return joined, postJSON(ctx, rawURL, map[string]any{"mobiles": mobiles, "content": body}, headers)
}

func headerMap(cfg map[string]any) map[string]string {
	raw, ok := cfg["headers"].(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]string{}
	for key, value := range raw {
		out[key] = validate.AsText(value)
	}
	return out
}

func jsonID(value string) any {
	if number, err := strconv.ParseInt(value, 10, 64); err == nil {
		return number
	}
	return value
}

func postJSON(ctx context.Context, rawURL string, payload any, headers map[string]string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["Content-Type"]; !ok {
		headers["Content-Type"] = "application/json"
	}
	return postRaw(ctx, rawURL, headers["Content-Type"], body, headers)
}

func postRaw(ctx context.Context, rawURL, contentType string, body []byte, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		if key == "Content-Type" {
			continue
		}
		req.Header.Set(key, value)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, limits.AlertReplyMax))
	if res.StatusCode >= 300 {
		return fmt.Errorf("对方返回 %d %s", res.StatusCode, validate.Clip(strings.TrimSpace(string(raw)), 180))
	}
	if bytes.Contains(raw, []byte(`"errcode"`)) && !bytes.Contains(raw, []byte(`"errcode":0`)) && !bytes.Contains(raw, []byte(`"errcode": 0`)) {
		return fmt.Errorf("对方返回 %s", validate.Clip(strings.TrimSpace(string(raw)), 180))
	}
	return nil
}
