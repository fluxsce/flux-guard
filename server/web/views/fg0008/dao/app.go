package dao

import (
	"context"
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
		SELECT instId, agentId, appCode, COALESCE(machineId, ''), COALESCE(hostName, ''), instanceName,
		       runtimeKind, COALESCE(modeCode, ''), artifactKind, COALESCE(versionText, ''), COALESCE(tomcatVersion, ''),
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
			&item.InstID, &item.AgentID, &item.AppCode, &item.MachineID, &item.HostName, &item.InstanceName,
			&item.RuntimeKind, &item.ModeCode, &item.ArtifactKind, &item.VersionText, &item.TomcatVersion,
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
		SELECT releaseId, appCode, artifactKind, versionText, fileName, addTime, addWho
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
		if err := rows.Scan(&item.ReleaseID, &item.AppCode, &item.ArtifactKind, &item.VersionText, &item.FileName, &added, &item.AddWho); err != nil {
			return nil, err
		}
		item.AddTime = store.FormatTime(timeFrom(added))
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
