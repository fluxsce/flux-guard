package dao

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"fluxguard/server/utils/const/limits"
	"fluxguard/server/store"
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
	rows, err := db.Query(ctx, `
		SELECT a.appCode, a.appName, a.artifactKind
		FROM FG_APP a`+where+`
		ORDER BY a.appName, a.appCode`, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var apps []models.App
	for rows.Next() {
		var item models.App
		if err := rows.Scan(&item.AppCode, &item.AppName, &item.ArtifactKind); err != nil {
			return nil, err
		}
		apps = append(apps, item)
	}
	if err := rows.Err(); err != nil {
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
	all, err := listInst(ctx, db, tenantID, "")
	if err != nil {
		return "", err
	}
	for _, row := range all {
		if row.AgentID == agentID && row.InstanceName == instanceName {
			return row.ModeCode, nil
		}
	}
	return "", ErrMissing
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
	rows, err := db.Query(ctx, `
		SELECT instId, agentId, appCode, COALESCE(machineId, ''), COALESCE(hostName, ''), COALESCE(ipAddress, ''),
		       instanceName, runtimeKind, COALESCE(modeCode, ''), artifactKind, COALESCE(artifactPath, ''),
		       COALESCE(versionText, ''), COALESCE(tomcatVersion, ''), COALESCE(commandAddress, ''),
		       runningFlag, reportedTime
		FROM FG_APP_INST`+where+`
		ORDER BY instanceName, instId`, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Instance
	now := time.Now()
	for rows.Next() {
		var item models.Instance
		var reported any
		if err := rows.Scan(
			&item.InstID, &item.AgentID, &item.AppCode, &item.MachineID, &item.HostName, &item.IPAddress,
			&item.InstanceName, &item.RuntimeKind, &item.ModeCode, &item.ArtifactKind, &item.ArtifactPath,
			&item.VersionText, &item.TomcatVersion, &item.CommandAddress,
			&item.RunningFlag, &reported,
		); err != nil {
			return nil, err
		}
		item.OnlineFlag = "N"
		if at, ok := store.TimeValue(reported); ok && now.Sub(at) <= limits.OnlineWindow {
			item.OnlineFlag = "Y"
		}
		item.ReportedTime = store.FormatTime(timeFrom(reported))
		out = append(out, item)
	}
	if out == nil {
		out = []models.Instance{}
	}
	return out, rows.Err()
}

func timeFrom(value any) time.Time {
	at, ok := store.TimeValue(value)
	if !ok {
		return time.Time{}
	}
	return at
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
	rows, err := db.Query(ctx, `
		SELECT releaseId, appCode, artifactKind, versionText, fileName, addTime, addWho, COALESCE(extProperty, '')
		FROM FG_APP_RELEASE
		WHERE tenantId = @tenantId AND appCode = @appCode AND activeFlag = 'Y'
		ORDER BY addTime DESC, releaseId DESC
		LIMIT `+strconv.Itoa(limit), dbx.Args{"tenantId": tenantID, "appCode": appCode})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Release
	for rows.Next() {
		var item models.Release
		var added any
		var extRaw string
		if err := rows.Scan(&item.ReleaseID, &item.AppCode, &item.ArtifactKind, &item.VersionText, &item.FileName, &added, &item.AddWho, &extRaw); err != nil {
			return nil, err
		}
		item.AddTime = store.FormatTime(timeFrom(added))
		var ext struct {
			Purpose         string `json:"purpose"`
			SourceReleaseID string `json:"sourceReleaseId"`
			PendingInstIDs  []string `json:"pendingInstIds"`
		}
		_ = json.Unmarshal([]byte(extRaw), &ext)
		item.Purpose = ext.Purpose
		item.SourceReleaseID = ext.SourceReleaseID
		item.PendingCount = len(ext.PendingInstIDs)
		out = append(out, item)
	}
	if out == nil {
		out = []models.Release{}
	}
	return out, rows.Err()
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
