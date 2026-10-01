// Package service 是组件监控给控制台用的门面。采集器在本包初始化时登记。
package service

import (
	"context"
	"fmt"
	"strconv"

	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/component/row"
	"fluxguard/server/component/runtime"
	"fluxguard/server/component/series"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"

	_ "fluxguard/server/component/kafka"
	_ "fluxguard/server/component/mongo"
	_ "fluxguard/server/component/mysql"
	_ "fluxguard/server/component/nacos"
	_ "fluxguard/server/component/redis"
	_ "fluxguard/server/component/zk"
)

// Catalog 返回代码里的组件目录。
func Catalog() []models.Kind {
	src := catalog.All()
	out := make([]models.Kind, 0, len(src))
	for _, item := range src {
		kind := models.Kind{
			KindCode: item.KindCode, KindName: item.KindName, DefaultPort: item.DefaultPort,
			MetricTable: item.MetricTable, SortOrder: item.SortOrder, NoteText: item.NoteText,
		}
		for _, metric := range item.Metrics {
			kind.Metrics = append(kind.Metrics, models.Metric{Code: metric.Code, Name: metric.Name, Unit: metric.Unit})
		}
		out = append(out, kind)
	}
	return out
}

// List 分页列出本租户连接。
func List(ctx context.Context, db *dbx.DB, filter conn.Filter, page dbx.Page) ([]models.Conn, int64, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return nil, 0, err
	}
	return conn.List(ctx, db, sess.TenantID, filter, page)
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
	kind, ok := catalog.Get(item.KindCode)
	if !ok || kind.MetricTable == "" {
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
	return series.Read(ctx, db, kind.MetricTable, sess.TenantID, componentID, catalog.Columns(item.KindCode), limit, span)
}
