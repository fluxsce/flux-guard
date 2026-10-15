package dao

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"fluxguard/server/store"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/web/views/fg0008/models"
	dbx "github.com/fluxsce/dbx/db"
)

// ListApps 返回未忽略的应用。版本不一致时 MixedVersion 为 Y。
func ListApps(ctx context.Context, db *dbx.DB, tenantID, appName string) ([]models.App, error) {
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE a.tenantId = @tenantId AND a.activeFlag = 'Y' AND a.ignoreFlag = 'N'"
	if name := strings.TrimSpace(appName); name != "" {
		where += " AND a.appName LIKE @appName"
		args["appName"] = "%" + name + "%"
	}
	cols, err := dbx.Columns(&models.App{})
	if err != nil {
		return nil, err
	}
	var apps []models.App
	if err := db.Select(ctx, &apps, `
		SELECT `+qualifyCols("a", cols)+`
		FROM FG_APP a`+where+`
		ORDER BY a.appName, a.appCode`, args); err != nil {
		return nil, err
	}
	if apps == nil {
		apps = []models.App{}
	}
	inst, err := listInst(ctx, db, tenantID, "")
	if err != nil {
		return nil, err
	}
	byApp := map[string][]models.Instance{}
	for _, item := range inst {
		byApp[item.AppCode] = append(byApp[item.AppCode], item)
	}
	for i := range apps {
		rows := byApp[apps[i].AppCode]
		apps[i].InstanceCount = len(rows)
		version := ""
		mixed := "N"
		online := 0
		for _, row := range rows {
			if row.OnlineFlag == "Y" {
				online++
			}
			if version == "" {
				version = row.VersionText
				continue
			}
			if row.VersionText != version {
				mixed = "Y"
			}
		}
		apps[i].VersionText = version
		apps[i].MixedVersion = mixed
		apps[i].OnlineCount = online
	}
	return apps, nil
}

// ListSites 按代理 + 实例名汇总运行时套件（左侧导航）。
func ListSites(ctx context.Context, db *dbx.DB, tenantID, keyword string) ([]models.Site, error) {
	all, err := listInst(ctx, db, tenantID, "")
	if err != nil {
		return nil, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	type key struct{ agent, inst string }
	order := make([]key, 0)
	by := map[key]*models.Site{}
	for _, row := range all {
		k := key{row.AgentID, row.InstanceName}
		site, ok := by[k]
		if !ok {
			site = &models.Site{
				AgentID: row.AgentID, InstanceName: row.InstanceName,
				HostName: row.HostName, IPAddress: row.IPAddress,
				ModeCode: row.ModeCode, RuntimeKind: row.RuntimeKind,
				CommandAddress: row.CommandAddress, TomcatVersion: row.TomcatVersion,
				ReportedTime: row.ReportedTime, OnlineFlag: row.OnlineFlag,
				BasePath: siteBase(row),
			}
			by[k] = site
			order = append(order, k)
		}
		if row.ArtifactKind != "runtime" {
			site.FileCount++
		}
		if row.OnlineFlag == "Y" {
			site.OnlineFlag = "Y"
		}
		if row.ReportedTime > site.ReportedTime {
			site.ReportedTime = row.ReportedTime
		}
		if bp := siteBase(row); bp != "" && (site.BasePath == "" || row.ArtifactKind == "runtime") {
			site.BasePath = bp
		}
		if site.CommandAddress == "" {
			site.CommandAddress = row.CommandAddress
		}
		if site.TomcatVersion == "" {
			site.TomcatVersion = row.TomcatVersion
		}
	}
	out := make([]models.Site, 0, len(order))
	for _, k := range order {
		site := by[k]
		if keyword != "" {
			blob := strings.ToLower(site.InstanceName + " " + site.HostName + " " + site.IPAddress + " " + site.ModeCode + " " + site.BasePath)
			if !strings.Contains(blob, keyword) {
				continue
			}
		}
		out = append(out, *site)
	}
	if out == nil {
		out = []models.Site{}
	}
	return out, nil
}

// ListSiteFiles 返回一套运行时下的制品文件（右侧文件仓）。不返回 files 心跳站点标记。
func ListSiteFiles(ctx context.Context, db *dbx.DB, tenantID, agentID, instanceName string) ([]models.Instance, error) {
	agentID = strings.TrimSpace(agentID)
	instanceName = strings.TrimSpace(instanceName)
	if agentID == "" || instanceName == "" {
		return nil, ErrMissing
	}
	all, err := listInst(ctx, db, tenantID, "")
	if err != nil {
		return nil, err
	}
	var out []models.Instance
	for _, row := range all {
		if row.AgentID == agentID && row.InstanceName == instanceName && row.ArtifactKind != "runtime" {
			out = append(out, row)
		}
	}
	if out == nil {
		out = []models.Instance{}
	}
	return out, nil
}

// SiteMode 读一套运行时的 mode（取任意一行）。
func SiteMode(ctx context.Context, db *dbx.DB, tenantID, agentID, instanceName string) (string, error) {
	row, err := SiteAnchor(ctx, db, tenantID, agentID, instanceName)
	if err != nil {
		return "", err
	}
	return row.ModeCode, nil
}

// SiteAnchor 取一套运行时的锚点行：优先 artifactKind=runtime，否则任意一行。
// 用于新增制品时挂发布明细与下发 Instance。
func SiteAnchor(ctx context.Context, db *dbx.DB, tenantID, agentID, instanceName string) (models.Instance, error) {
	agentID = strings.TrimSpace(agentID)
	instanceName = strings.TrimSpace(instanceName)
	if agentID == "" || instanceName == "" {
		return models.Instance{}, ErrMissing
	}
	all, err := listInst(ctx, db, tenantID, "")
	if err != nil {
		return models.Instance{}, err
	}
	var fallback models.Instance
	for _, row := range all {
		if row.AgentID != agentID || row.InstanceName != instanceName {
			continue
		}
		if row.ArtifactKind == "runtime" {
			return row, nil
		}
		if fallback.InstID == "" {
			fallback = row
		}
	}
	if fallback.InstID != "" {
		return fallback, nil
	}
	return models.Instance{}, ErrMissing
}

func siteBase(row models.Instance) string {
	path := strings.TrimSpace(row.ArtifactPath)
	if path == "" {
		return ""
	}
	// files 站点标记的 path 就是 base，不要再剥一层。
	if row.ModeCode == "files" && row.ArtifactKind == "runtime" {
		return path
	}
	// Tomcat 展示 CATALINA_HOME（webapps / logs / conf 的上一级），便于进 logs 看启动日志。
	if row.ModeCode == "tomcat" {
		if home := tomcatHomeOf(path); home != "" {
			return home
		}
	}
	return baseOf(path, row.ModeCode)
}

func baseOf(path, mode string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	// files / tomcat / node 等：展示目录；jar 仍展示文件所在目录。
	sep := "/"
	if strings.Contains(path, `\`) {
		sep = `\`
	}
	i := strings.LastIndex(path, sep)
	if i <= 0 {
		return path
	}
	if mode == "jar" || mode == "static" {
		return path[:i]
	}
	return path[:i]
}

// tomcatHomeOf 从 webapps/WAR 或 logs 下的文件路径还原 CATALINA_HOME。
// 不用 filepath.ToSlash：Linux 流水线上 Separator 是 /，无法把 Windows 反斜杠路径归一化。
func tomcatHomeOf(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	slash := strings.ReplaceAll(path, `\`, "/")
	lower := strings.ToLower(slash)
	for _, marker := range []string{"/webapps/", "/logs/", "/conf/", "/bin/"} {
		if i := strings.Index(lower, marker); i > 0 {
			home := slash[:i]
			if strings.Contains(path, `\`) {
				return strings.ReplaceAll(home, "/", `\`)
			}
			return home
		}
	}
	return ""
}

// GetInstance 按实例号取一行。
func GetInstance(ctx context.Context, db *dbx.DB, tenantID, instID string) (models.Instance, error) {
	instID = strings.TrimSpace(instID)
	if instID == "" {
		return models.Instance{}, ErrMissing
	}
	all, err := listInst(ctx, db, tenantID, "")
	if err != nil {
		return models.Instance{}, err
	}
	for _, row := range all {
		if row.InstID == instID {
			return row, nil
		}
	}
	return models.Instance{}, ErrMissing
}

// ListInstances 返回一个应用下仍在上报的实例。
func ListInstances(ctx context.Context, db *dbx.DB, tenantID, appCode string) ([]models.Instance, error) {
	return listInst(ctx, db, tenantID, appCode)
}

func listInst(ctx context.Context, db *dbx.DB, tenantID, appCode string) ([]models.Instance, error) {
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE tenantId = @tenantId AND activeFlag = 'Y'"
	if appCode != "" {
		where += " AND appCode = @appCode"
		args["appCode"] = appCode
	}
	cols, err := dbx.Columns(&models.Instance{})
	if err != nil {
		return nil, err
	}
	var out []models.Instance
	if err := db.Select(ctx, &out, `
		SELECT `+cols+`
		FROM FG_APP_INST`+where+`
		ORDER BY instanceName, instId`, args); err != nil {
		return nil, err
	}
	now := time.Now()
	for i := range out {
		out[i].OnlineFlag = "N"
		if at, ok := store.ParseTime(out[i].ReportedTime); ok && now.Sub(at) <= limits.OnlineWindow {
			out[i].OnlineFlag = "Y"
			out[i].ReportedTime = store.FormatTime(at)
		}
	}
	if out == nil {
		out = []models.Instance{}
	}
	return out, nil
}

// qualifyCols 给逗号分隔列名加表别名前缀（如 a.appCode）。
func qualifyCols(alias, cols string) string {
	parts := strings.Split(cols, ", ")
	for i, p := range parts {
		parts[i] = alias + "." + p
	}
	return strings.Join(parts, ", ")
}

// Rename 只改显示名。应用编码仍是上报的名字。
func Rename(ctx context.Context, db *dbx.DB, tenantID, appCode, appName, user string) error {
	res, err := db.Exec(ctx, `
		UPDATE FG_APP SET appName = @appName, editTime = @editTime, editWho = @editWho,
		  currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND appCode = @appCode AND activeFlag = 'Y'`,
		dbx.Args{
			"appName": appName, "editTime": store.FormatTime(time.Now().UTC()), "editWho": user,
			"tenantId": tenantID, "appCode": appCode,
		})
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrMissing
	}
	return nil
}

// ListReleases 返回一个应用最近的发布记录，不含各实例明细。
func ListReleases(ctx context.Context, db *dbx.DB, tenantID, appCode string, limit int) ([]models.Release, error) {
	appCode = strings.TrimSpace(appCode)
	if appCode == "" {
		return nil, ErrMissing
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	cols, err := dbx.Columns(&models.Release{})
	if err != nil {
		return nil, err
	}
	var rows []models.Release
	if err := db.Select(ctx, &rows, `
		SELECT `+cols+`
		FROM FG_APP_RELEASE
		WHERE tenantId = @tenantId AND appCode = @appCode AND activeFlag = 'Y'
		ORDER BY addTime DESC, releaseId DESC
		LIMIT `+strconv.Itoa(limit), dbx.Args{"tenantId": tenantID, "appCode": appCode}); err != nil {
		return nil, err
	}
	for i := range rows {
		var ext struct {
			Purpose         string   `json:"purpose"`
			SourceReleaseID string   `json:"sourceReleaseId"`
			PendingInstIDs  []string `json:"pendingInstIds"`
		}
		_ = json.Unmarshal([]byte(rows[i].ExtProperty), &ext)
		rows[i].Purpose = ext.Purpose
		rows[i].SourceReleaseID = ext.SourceReleaseID
		rows[i].PendingCount = len(ext.PendingInstIDs)
		rows[i].TenantID = tenantID
	}
	if rows == nil {
		rows = []models.Release{}
	}
	return rows, nil
}

// Ignore 让这个应用以后不再出现在列表里。上报不会把它改回来。
func Ignore(ctx context.Context, db *dbx.DB, tenantID, appCode, user string) error {
	res, err := db.Exec(ctx, `
		UPDATE FG_APP SET ignoreFlag = 'Y', editTime = @editTime, editWho = @editWho,
		  currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND appCode = @appCode AND activeFlag = 'Y'`,
		dbx.Args{
			"editTime": store.FormatTime(time.Now().UTC()), "editWho": user,
			"tenantId": tenantID, "appCode": appCode,
		})
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrMissing
	}
	return nil
}
