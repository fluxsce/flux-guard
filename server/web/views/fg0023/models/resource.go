package models

// Resource 是 FG_AUTH_RESOURCE 的一行。后续新增、修改从这里扩字段。
type Resource struct {
	ResourceID       string
	TenantID         string
	ResourceName     string
	ResourceCode     string
	ResourceType     string
	ResourcePath     string
	ParentResourceID string
	ResourceLevel    int
	SortOrder        int
	ResourceStatus   string
	ActiveFlag       string
	AddTime          string
	AddWho           string
	EditTime         string
	EditWho          string
}

// Node 是目录页返回的树节点。
type Node struct {
	ResourceID   string  `json:"resourceId"`
	ResourceName string  `json:"resourceName"`
	ResourceCode string  `json:"resourceCode"`
	ResourceType string  `json:"resourceType"`
	ResourcePath string  `json:"resourcePath,omitempty"`
	AddTime      string  `json:"addTime,omitempty"`
	AddWho       string  `json:"addWho,omitempty"`
	EditTime     string  `json:"editTime,omitempty"`
	EditWho      string  `json:"editWho,omitempty"`
	Children     []*Node `json:"children,omitempty"`
}

// Detail 把一行收成查看结果。不含下级，和列表树分开。
func Detail(row *Resource) *Node {
	if row == nil {
		return nil
	}
	return &Node{
		ResourceID:   row.ResourceID,
		ResourceName: row.ResourceName,
		ResourceCode: row.ResourceCode,
		ResourceType: row.ResourceType,
		ResourcePath: row.ResourcePath,
		AddTime:      row.AddTime,
		AddWho:       row.AddWho,
		EditTime:     row.EditTime,
		EditWho:      row.EditWho,
	}
}

// Tree 按查询顺序把平铺资源收成树。父节点不在结果里时，该节点自己作为根。
func Tree(rows []*Resource) []*Node {
	nodes := make(map[string]*Node, len(rows))
	order := make([]string, 0, len(rows))
	children := make(map[string][]string)
	parentOf := make(map[string]string, len(rows))
	for _, row := range rows {
		if row == nil || row.ResourceID == "" {
			continue
		}
		nodes[row.ResourceID] = &Node{
			ResourceID:   row.ResourceID,
			ResourceName: row.ResourceName,
			ResourceCode: row.ResourceCode,
			ResourceType: row.ResourceType,
			ResourcePath: row.ResourcePath,
			AddTime:      row.AddTime,
			AddWho:       row.AddWho,
			EditTime:     row.EditTime,
			EditWho:      row.EditWho,
		}
		order = append(order, row.ResourceID)
		parentOf[row.ResourceID] = row.ParentResourceID
		if row.ParentResourceID != "" {
			children[row.ParentResourceID] = append(children[row.ParentResourceID], row.ResourceID)
		}
	}
	var roots []*Node
	for _, id := range order {
		node := nodes[id]
		for _, childID := range children[id] {
			child, ok := nodes[childID]
			if !ok {
				continue
			}
			node.Children = append(node.Children, child)
		}
		parent := parentOf[id]
		if parent == "" || nodes[parent] == nil {
			roots = append(roots, node)
		}
	}
	return roots
}
