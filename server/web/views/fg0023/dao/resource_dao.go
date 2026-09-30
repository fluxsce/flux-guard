package dao

import (
	"context"
	"strings"

	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0023/models"
	dbx "github.com/fluxsce/dbx/db"
)

const resourceColumns = `
	resourceId, tenantId, resourceName, resourceCode, resourceType,
	COALESCE(resourcePath, ''), COALESCE(parentResourceId, ''),
	resourceLevel, sortOrder, resourceStatus, activeFlag,
	addTime, addWho, editTime, editWho`

// Filter 是资源列表的筛选条件。空条件返回全部启用资源。
type Filter struct {
	ResourceName string
	ResourceCode string
	ResourceType string
}

func (f Filter) active() bool {
	return strings.TrimSpace(f.ResourceName) != "" ||
		strings.TrimSpace(f.ResourceCode) != "" ||
		strings.TrimSpace(f.ResourceType) != ""
}

// ResourceDAO 读写 FG_AUTH_RESOURCE。连接由鉴权服务交出，本模块不另开库。
type ResourceDAO struct {
	db *dbx.DB
}

// NewResourceDAO 创建资源数据访问。
func NewResourceDAO(db *dbx.DB) *ResourceDAO {
	return &ResourceDAO{db: db}
}

// List 按名称、编码、类型查询本租户启用的资源。
// 命中节点保留上级，并带上自己的下级，顺序仍可直接组树。
func (d *ResourceDAO) List(ctx context.Context, tenantID string, filter Filter) ([]*models.Resource, error) {
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE tenantId = @tenantId AND activeFlag = 'Y' AND resourceStatus = 'Y'"
	where += store.LikeClause(args, "resourceName", "nameLike", filter.ResourceName)
	where += store.LikeClause(args, "resourceCode", "codeLike", filter.ResourceCode)
	where += store.EqClause(args, "resourceType", "resourceType", filter.ResourceType)
	matched, err := d.query(ctx, where, args)
	if err != nil || !filter.active() {
		return matched, err
	}
	all, err := d.query(ctx, " WHERE tenantId = @tenantId AND activeFlag = 'Y' AND resourceStatus = 'Y'", dbx.Args{"tenantId": tenantID})
	if err != nil {
		return nil, err
	}
	return closeTree(all, matched), nil
}

// Get 按资源主键精确读取一行。不存在时返回 sql.ErrNoRows。
func (d *ResourceDAO) Get(ctx context.Context, tenantID, resourceID string) (*models.Resource, error) {
	item := &models.Resource{}
	err := store.Scan(ctx, d.db, `
		SELECT `+resourceColumns+`
		FROM FG_AUTH_RESOURCE
		WHERE tenantId = @tenantId AND resourceId = @resourceId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "resourceId": resourceID},
		&item.ResourceID, &item.TenantID, &item.ResourceName, &item.ResourceCode, &item.ResourceType,
		&item.ResourcePath, &item.ParentResourceID, &item.ResourceLevel, &item.SortOrder,
		&item.ResourceStatus, &item.ActiveFlag,
		&item.AddTime, &item.AddWho, &item.EditTime, &item.EditWho,
	)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (d *ResourceDAO) query(ctx context.Context, where string, args dbx.Args) ([]*models.Resource, error) {
	rows, err := d.db.Query(ctx, `
		SELECT `+resourceColumns+`
		FROM FG_AUTH_RESOURCE`+where+`
		ORDER BY resourceLevel, sortOrder, resourceCode`, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Resource
	for rows.Next() {
		item := &models.Resource{}
		if err := rows.Scan(
			&item.ResourceID, &item.TenantID, &item.ResourceName, &item.ResourceCode, &item.ResourceType,
			&item.ResourcePath, &item.ParentResourceID, &item.ResourceLevel, &item.SortOrder,
			&item.ResourceStatus, &item.ActiveFlag,
			&item.AddTime, &item.AddWho, &item.EditTime, &item.EditWho,
		); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

// closeTree 把命中行补回上级和下级，避免列表筛掉后树断开。
func closeTree(all, matched []*models.Resource) []*models.Resource {
	byID := make(map[string]*models.Resource, len(all))
	children := make(map[string][]string)
	for _, row := range all {
		if row == nil || row.ResourceID == "" {
			continue
		}
		byID[row.ResourceID] = row
		if row.ParentResourceID != "" {
			children[row.ParentResourceID] = append(children[row.ParentResourceID], row.ResourceID)
		}
	}
	keep := map[string]struct{}{}
	visited := map[string]struct{}{}
	var down func(id string)
	down = func(id string) {
		if _, ok := visited[id]; ok {
			return
		}
		visited[id] = struct{}{}
		if _, ok := byID[id]; !ok {
			return
		}
		keep[id] = struct{}{}
		for _, childID := range children[id] {
			down(childID)
		}
	}
	for _, row := range matched {
		if row == nil {
			continue
		}
		down(row.ResourceID)
		seen := map[string]struct{}{}
		for parent := byID[row.ParentResourceID]; parent != nil; parent = byID[parent.ParentResourceID] {
			if _, ok := seen[parent.ResourceID]; ok {
				break
			}
			seen[parent.ResourceID] = struct{}{}
			keep[parent.ResourceID] = struct{}{}
		}
	}
	out := make([]*models.Resource, 0, len(keep))
	for _, row := range all {
		if row == nil {
			continue
		}
		if _, ok := keep[row.ResourceID]; ok {
			out = append(out, row)
		}
	}
	return out
}
