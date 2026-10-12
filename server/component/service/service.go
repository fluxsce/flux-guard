// Package service 是组件监控给控制台用的门面。采集器在本包初始化时登记。
package service

import (
	"context"
	"fmt"
	"strconv"

	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/component/pane"
	"fluxguard/server/component/panel"
	"fluxguard/server/component/row"
	"fluxguard/server/component/runtime"
	"fluxguard/server/component/series"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"

	_ "fluxguard/server/component/mongo"
	_ "fluxguard/server/component/mysql"
	_ "fluxguard/server/component/nacos"
	_ "fluxguard/server/component/redis"
	_ "fluxguard/server/component/zk"
)

// Catalog 返回本租户在库里的组件目录，并带上本进程的指标名。
func Catalog(ctx context.Context, db *dbx.DB) ([]models.Kind, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, fmt.Errorf("数据库未打开")
	}
	rows, err := db.Query(ctx, `
		SELECT kindCode, kindName, defaultPort, metricTable, sortOrder, COALESCE(noteText, '')
		FROM FG_COMPONENT_KIND
		WHERE tenantId = @tenantId AND activeFlag = 'Y' AND statusFlag = 'Y'
		ORDER BY sortOrder, kindCode`, dbx.Args{"tenantId": sess.TenantID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Kind
	for rows.Next() {
		var code, name, table, note, port, sort any
		if err := rows.Scan(&code, &name, &port, &table, &sort, &note); err != nil {
			rows.Close()
			return nil, err
		}
		kindCode := row.Text(code)
		if !catalog.Known(kindCode) {
			continue
		}
		kind := models.Kind{
			KindCode: kindCode, KindName: row.Text(name), DefaultPort: int(row.Float(port)),
			MetricTable: row.Text(table), SortOrder: int(row.Float(sort)), NoteText: row.Text(note),
		}
		for _, metric := range catalog.Metrics(kindCode) {
			kind.Metrics = append(kind.Metrics, models.Metric{Code: metric.Code, Name: metric.Name, Unit: metric.Unit})
		}
		out = append(out, kind)
	}
	scanErr := rows.Err()
	rows.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	// 开发库 SQLite 只有一条连接。种类游标没关时再查页签，会一直等这张连接，接口直到前端超时。
	for i := range out {
		views, err := panel.Of(ctx, db, sess.TenantID, out[i].KindCode)
		if err != nil {
			return nil, err
		}
		for _, item := range views {
			view := models.Panel{
				PanelCode: item.PanelCode, PanelName: item.PanelName, SortOrder: item.SortOrder,
				TerminalFlag: "N", ListFlag: "N", Metrics: item.Metrics, Sections: item.Sections,
			}
			if item.Terminal {
				view.TerminalFlag = "Y"
			}
			if item.List {
				view.ListFlag = "Y"
			}
			out[i].Panels = append(out[i].Panels, view)
		}
	}
	if out == nil {
		out = []models.Kind{}
	}
	return out, nil
}

// List 分页列出本租户连接。
func List(ctx context.Context, db *dbx.DB, filter conn.Filter, page dbx.Page) ([]models.Conn, int64, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, total, err := conn.List(ctx, db, sess.TenantID, filter, page)
	if err != nil {
		return nil, 0, err
	}
	if err := attachHeadlines(ctx, db, sess.TenantID, filter.KindCode, rows); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Lookup 按编号取一条连接，供进入详情前核对种类权限。
func Lookup(ctx context.Context, db *dbx.DB, componentID string) (models.Conn, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return models.Conn{}, err
	}
	return conn.Get(ctx, db, sess.TenantID, componentID)
}

func attachHeadlines(ctx context.Context, db *dbx.DB, tenantID, kind string, rows []models.Conn) error {
	cols, err := panel.ListMetrics(ctx, db, tenantID, kind)
	if err != nil {
		return err
	}
	if len(cols) == 0 || len(rows) == 0 {
		return nil
	}
	table, ok := catalog.PaneTable(kind)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(rows))
	for _, item := range rows {
		if item.ComponentID != "" {
			ids = append(ids, item.ComponentID)
		}
	}
	found, err := pane.Latest(ctx, db, table, tenantID, ids, cols)
	if err != nil {
		return err
	}
	for i := range rows {
		if values, ok := found[rows[i].ComponentID]; ok {
			rows[i].Headlines = values
		}
	}
	return nil
}

// Save 新增或修改连接。
func Save(ctx context.Context, db *dbx.DB, in models.Input, creating bool) (models.Conn, error) {
	return conn.Save(ctx, db, in, creating)
}

// Delete 停用连接。
func Delete(ctx context.Context, db *dbx.DB, componentID string) error {
	return conn.Delete(ctx, db, componentID)
}

// Test 按弹窗填写拨一次号，不写入连接和采集表。
func Test(ctx context.Context, db *dbx.DB, in models.Input) (models.Trial, error) {
	target, err := conn.Draft(ctx, db, in)
	if err != nil {
		return models.Trial{}, err
	}
	status, err := runtime.Probe(ctx, nil, target)
	if err != nil {
		return models.Trial{}, err
	}
	return models.Trial{
		UpFlag: row.Flag(status.Up), VersionText: status.Version, RoleName: status.Role,
		ErrorText: status.Error, SummaryText: status.Summary,
	}, nil
}

// Probe 立刻拨一次，并把最新连接行返回。拨不通时连接行已记下原因。
func Probe(ctx context.Context, db *dbx.DB, componentID string) (models.Conn, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return models.Conn{}, err
	}
	target, err := conn.LoadOpen(ctx, db, sess.TenantID, componentID)
	if err != nil {
		return models.Conn{}, err
	}
	if _, err := runtime.Probe(ctx, db, target); err != nil {
		item, getErr := conn.Get(ctx, db, sess.TenantID, componentID)
		if getErr != nil {
			return models.Conn{}, err
		}
		return item, nil
	}
	return conn.Get(ctx, db, sess.TenantID, componentID)
}

// Latest 返回最近一个采集点。
func Latest(ctx context.Context, db *dbx.DB, componentID string) (models.Latest, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return models.Latest{}, err
	}
	item, err := conn.Get(ctx, db, sess.TenantID, componentID)
	if err != nil {
		return models.Latest{}, err
	}
	points, err := runtime.Series(ctx, db, item.KindCode, sess.TenantID, componentID, 1)
	if err != nil {
		return models.Latest{}, err
	}
	out := models.Latest{ComponentID: item.ComponentID, KindCode: item.KindCode}
	if len(points) > 0 {
		point := points[len(points)-1]
		out.Point = &point
	}
	return out, nil
}

// Series 返回曲线点。未给时间窗时按条数取最近一批；给出起止时间时按观测时间过滤，最多 4000 点。
func Series(ctx context.Context, db *dbx.DB, componentID, limitText, fromText, toText string) ([]models.Point, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return nil, err
	}
	item, err := conn.Get(ctx, db, sess.TenantID, componentID)
	if err != nil {
		return nil, err
	}
	table, ok := catalog.MetricTable(item.KindCode)
	if !ok {
		return nil, fmt.Errorf("这种组件还没有采集表")
	}
	span := series.Span{}
	if from, ok := store.ParseTime(fromText); ok {
		span.From = from
	}
	if to, ok := store.ParseTime(toText); ok {
		span.To = to
	}
	limit, _ := strconv.Atoi(limitText)
	return series.Read(ctx, db, table, sess.TenantID, componentID, catalog.Columns(item.KindCode), limit, span)
}
