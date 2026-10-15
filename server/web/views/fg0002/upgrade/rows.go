package upgrade

import (
	"encoding/json"
	"strings"
	"time"

	"fluxguard/server/command/models"
	"fluxguard/server/store"
	fgmodels "fluxguard/server/web/views/fg0002/models"
)

// agentPickRow 用于校验待升级代理是否在线且平台匹配（非 API 模型）。
type agentPickRow struct {
	AgentID      string `db:"agentId"`
	AgentCode    string `db:"agentCode"`
	StatusFlag   string `db:"statusFlag"`
	LastSeenTime string `db:"lastSeenTime"`
	GoOS         string `db:"goos"`
	GoArch       string `db:"goarch"`
}

// openFlagRow 探测某次升级是否仍有未结束命令。
type openFlagRow struct {
	StatusFlag string `db:"statusFlag"`
}

func parseExt(raw string) releaseExt {
	var ext releaseExt
	_ = json.Unmarshal([]byte(raw), &ext)
	return ext
}

func applyReleaseMeta(row *fgmodels.AgentRelease) releaseExt {
	ext := parseExt(row.ExtProperty)
	row.AddTime = formatStoredTime(row.AddTime)
	row.PendingCount = len(ext.PendingAgentIDs)
	row.BatchSize = ext.BatchSize
	if row.Items == nil {
		row.Items = []fgmodels.AgentReleaseItem{}
	}
	return ext
}

func finalizeItem(item *fgmodels.AgentReleaseItem, targetVersion string) {
	item.Status = models.StatusName(item.StatusFlag)
	item.VersionMatched = matchVersion(item.AgentVersion, targetVersion)
}

func finalizeStatus(item *fgmodels.AgentUpgradeStatus, rel fgmodels.AgentRelease) {
	item.ReleaseID = rel.ReleaseID
	item.VersionText = rel.VersionText
	item.Status = models.StatusName(item.StatusFlag)
	item.VersionMatched = matchVersion(item.AgentVersion, rel.VersionText)
}

func matchVersion(agentVersion, target string) string {
	if agentVersion == "" {
		return ""
	}
	if agentVersion == target {
		return "Y"
	}
	return "N"
}

func formatStoredTime(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if at, ok := store.ParseTime(raw); ok {
		return store.FormatTime(at)
	}
	return raw
}

func parseSeen(raw string) (time.Time, bool) {
	return store.ParseTime(strings.TrimSpace(raw))
}
