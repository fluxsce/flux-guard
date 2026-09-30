// Package plugin 是服务器的内置扩展。
// 菜单向权限目录声明资源，转发在批次写入近期样本之后再送出去。采集不在这个进程里。
package plugin

import (
	"context"

	"fluxguard/protocol"
)

// Sink 服务器宿主加载的转发扩展。近期样本由平台自己落库。
type Sink interface {
	Code() string
	Name() string
	Accept(ctx context.Context, batch protocol.Batch) error
}

// ResourceDecl 菜单扩展向权限目录声明的一条资源。编码规则与 FG_AUTH_RESOURCE 一致。
type ResourceDecl struct {
	ResourceID       string
	ResourceName     string
	ResourceCode     string
	ResourceType     string
	ResourcePath     string
	ResourceMethod   string
	ParentResourceID string
	ResourceLevel    int
	SortOrder        int
	IconClass        string
}

// Module 服务器宿主加载的菜单扩展。只声明菜单和按钮，不自己挂 HTTP。
type Module interface {
	Code() string
	Name() string
	Resources() []ResourceDecl
}

// Info 本进程已加载扩展的目录项，字段名与 FG_PLUGIN 对齐。
type Info struct {
	Code        string `json:"pluginCode"`
	Name        string `json:"pluginName"`
	Kind        string `json:"pluginKind"`
	Scope       string `json:"pluginScope"`
	BuiltinFlag string `json:"builtinFlag"`
}
