package upgrade

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	fgmodels "fluxguard/server/web/views/fg0002/models"

	dbx "github.com/fluxsce/dbx/db"
)

// List 返回本租户最近的代理升级记录（不含各台明细）。agentCode 为空则不限种类。
func List(ctx context.Context, db *dbx.DB, tenantID, agentCode string, limit int) ([]fgmodels.AgentRelease, error) {
	heads, err := listHeaders(ctx, db, tenantID, agentCode, limit)
	if err != nil {
		return nil, err
	}
	out := make([]fgmodels.AgentRelease, 0, len(heads))
	for _, head := range heads {
		out = append(out, head.rel)
	}
	if err := markOpen(ctx, db, tenantID, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Statuses 返回各代理在最近升级里仍需关注的状态（已对齐完成的不返回）。
func Statuses(ctx context.Context, db *dbx.DB, tenantID string) ([]fgmodels.AgentUpgradeStatus, error) {
	releases, err := listHeaders(ctx, db, tenantID, "", 30)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []fgmodels.AgentUpgradeStatus
	for _, head := range releases {
		for _, agentID := range head.pending {
			if seen[agentID] {
				continue
			}
			seen[agentID] = true
			out = append(out, fgmodels.AgentUpgradeStatus{
				AgentID: agentID, ReleaseID: head.rel.ReleaseID, VersionText: head.rel.VersionText, Status: "pending",
			})
		}
		items, err := itemStatuses(ctx, db, tenantID, head.rel)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if seen[item.AgentID] {
				continue
			}
			seen[item.AgentID] = true
			if item.Status == "done" && item.VersionMatched == "Y" {
				continue
			}
			out = append(out, item)
		}
	}
	if out == nil {
		out = []fgmodels.AgentUpgradeStatus{}
	}
	return out, nil
}

type releaseHeader struct {
	rel     fgmodels.AgentRelease
	pending []string
}

func listHeaders(ctx context.Context, db *dbx.DB, tenantID, agentCode string, limit int) ([]releaseHeader, error) {
	agentCode = strings.TrimSpace(agentCode)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	cols, err := dbx.Columns(&fgmodels.AgentRelease{})
	if err != nil {
		return nil, err
	}
	query := `
		SELECT ` + cols + `
		FROM FG_AGENT_RELEASE
		WHERE tenantId = @tenantId AND activeFlag = 'Y'`
	args := dbx.Args{"tenantId": tenantID}
	if agentCode != "" {
		query += ` AND agentCode = @agentCode`
		args["agentCode"] = agentCode
	}
	query += `
		ORDER BY addTime DESC, releaseId DESC
		LIMIT ` + strconv.Itoa(limit)
	var rows []fgmodels.AgentRelease
	if err := db.Select(ctx, &rows, query, args); err != nil {
		return nil, err
	}
	out := make([]releaseHeader, 0, len(rows))
	for i := range rows {
		rows[i].TenantID = tenantID
		ext := applyReleaseMeta(&rows[i])
		out = append(out, releaseHeader{rel: rows[i], pending: ext.PendingAgentIDs})
	}
	return out, nil
}

func markOpen(ctx context.Context, db *dbx.DB, tenantID string, rows []fgmodels.AgentRelease) error {
	for i := range rows {
		if rows[i].PendingCount > 0 {
			rows[i].OpenFlag = "Y"
			continue
		}
		open, err := hasOpenCommand(ctx, db, tenantID, rows[i].ReleaseID)
		if err != nil {
			return err
		}
		if open {
			rows[i].OpenFlag = "Y"
		} else {
			rows[i].OpenFlag = "N"
		}
	}
	return nil
}

func hasOpenCommand(ctx context.Context, db *dbx.DB, tenantID, releaseID string) (bool, error) {
	var row openFlagRow
	err := db.Get(ctx, &row, `
		SELECT cmd.statusFlag AS statusFlag
		FROM FG_AGENT_RELEASE_ITEM item
		JOIN FG_COMMAND cmd ON cmd.tenantId = item.tenantId AND cmd.commandId = item.commandId
		WHERE item.tenantId = @tenantId AND item.releaseId = @releaseId AND item.activeFlag = 'Y'
		  AND cmd.statusFlag IN ('D', 'W', 'R')
		LIMIT 1`,
		dbx.Args{"tenantId": tenantID, "releaseId": releaseID})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.StatusFlag != "", nil
}

func itemStatuses(ctx context.Context, db *dbx.DB, tenantID string, rel fgmodels.AgentRelease) ([]fgmodels.AgentUpgradeStatus, error) {
	var rows []fgmodels.AgentUpgradeStatus
	if err := db.Select(ctx, &rows, `
		SELECT item.agentId AS agentId, cmd.statusFlag AS statusFlag,
		       cmd.resultText AS resultText, a.agentVersion AS agentVersion
		FROM FG_AGENT_RELEASE_ITEM item
		JOIN FG_COMMAND cmd ON cmd.tenantId = item.tenantId AND cmd.commandId = item.commandId
		LEFT JOIN FG_AGENT a ON a.tenantId = item.tenantId AND a.agentId = item.agentId
		WHERE item.tenantId = @tenantId AND item.releaseId = @releaseId AND item.activeFlag = 'Y'
		ORDER BY item.addTime, item.commandId`,
		dbx.Args{"tenantId": tenantID, "releaseId": rel.ReleaseID}); err != nil {
		return nil, err
	}
	for i := range rows {
		finalizeStatus(&rows[i], rel)
	}
	return rows, nil
}
