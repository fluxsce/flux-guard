package dao

import (
	"context"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// noteJavaGeneration 按 JVM 启动时间划分进程代次。
// 启动时间没变就留在当前代。变了就收口上一代，从这次上报另开一代。
// 次数仍由后续样本写回当前代，收口时留下的是上一代最后一次累计。
func noteJavaGeneration(ctx context.Context, tx *dbx.DB, batch protocol.Batch, start any, processID string) error {
	next, ok := store.TimeValue(start)
	if !ok {
		return nil
	}
	opened := generationObserved(batch)
	var openCount int
	var openStart any
	if err := store.Scan(ctx, tx, `
		SELECT count(*), max(startTime)
		FROM FG_JAVA_RUN
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' AND closedTime IS NULL`,
		dbx.Args{"tenantId": batch.TenantID, "agentId": batch.AgentID},
		&openCount, &openStart); err != nil {
		return err
	}
	if openCount > 0 {
		if current, ok := store.TimeValue(openStart); ok && current.Equal(next) {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			UPDATE FG_JAVA_RUN SET
			  closedTime = @closedTime, editTime = @editTime, editWho = 'system',
			  currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' AND closedTime IS NULL`,
			dbx.Args{
				"closedTime": store.FormatTime(opened), "editTime": store.FormatTime(time.Now().UTC()),
				"tenantId": batch.TenantID, "agentId": batch.AgentID,
			}); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO FG_JAVA_RUN (
		  runId, agentId, tenantId, processId, startTime, openedTime,
		  collectionCount, collectionTimeMs,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @runId, @agentId, @tenantId, @processId, @startTime, @openedTime,
		  0, 0,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, dbx.Args{
		"runId": id.New(), "agentId": batch.AgentID, "tenantId": batch.TenantID,
		"processId": nullText(clip(processID, 32)), "startTime": store.FormatTime(next),
		"openedTime": store.FormatTime(opened), "addWho": batch.AgentID,
	})
	return err
}

func generationObserved(batch protocol.Batch) time.Time {
	var at time.Time
	for _, sample := range batch.Samples {
		if sample.ObservedAt.IsZero() {
			continue
		}
		if at.IsZero() || sample.ObservedAt.Before(at) {
			at = sample.ObservedAt
		}
	}
	if at.IsZero() {
		return batch.SentAt
	}
	return at
}
