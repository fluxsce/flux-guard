package routes

import (
	"context"
	"sort"
	"sync"

	"github.com/gin-gonic/gin"

	"fluxguard/common/logger"
	dbx "github.com/fluxsce/dbx/db"
)

// InitFunc 是模块路由注册函数。各模块在 init 里把自己登记进来，只接收路由和库连接。
type InitFunc func(router *gin.Engine, db *dbx.DB)

// ModuleInfo 是一个已登记的管理模块。
type ModuleInfo struct {
	Name     string
	BasePath string
	Init     InitFunc
}

// LoopFunc 是模块自己的后台循环。预警发送不在这里，各功能在循环里计算后把事件交给预警。
type LoopFunc func(ctx context.Context)

var (
	modules = map[string]*ModuleInfo{}
	entries []InitFunc
	loops   []LoopFunc
)

// RegisterModuleRoutes 在模块 routes 包的 init 中调用。目录名重复时 panic。
func RegisterModuleRoutes(moduleName string, init InitFunc) {
	if _, ok := modules[moduleName]; ok {
		panic("管理模块重复: " + moduleName)
	}
	modules[moduleName] = &ModuleInfo{
		Name:     moduleName,
		BasePath: ModuleAPIPrefix(moduleName),
		Init:     init,
	}
}

// RegisterLoop 登记模块后台循环。Mount 完成后再由 RunLoops 启动，取消 ctx 后循环应返回。
func RegisterLoop(fn LoopFunc) {
	if fn == nil {
		return
	}
	loops = append(loops, fn)
}

// RunLoops 启动已登记的后台循环。返回的通道在全部循环退出后关闭。
func RunLoops(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	var wg sync.WaitGroup
	for _, fn := range loops {
		wg.Add(1)
		go func(fn LoopFunc) {
			defer wg.Done()
			fn(ctx)
		}(fn)
	}
	go func() {
		wg.Wait()
		close(done)
	}()
	return done
}

// RegisterEntry 登记不进模块号的入口。用法和模块一样：routes 包的 init 里登记，Mount 时挂上。
func RegisterEntry(init InitFunc) {
	if init == nil {
		return
	}
	entries = append(entries, init)
}

// Modules 返回已登记模块的副本。
func Modules() []*ModuleInfo {
	out := make([]*ModuleInfo, 0, len(modules))
	for _, item := range modules {
		out = append(out, item)
	}
	return out
}

// Mount 挂健康检查、已登记入口和模块。调用前需要先导入 web/init。
// 会话和模块校验在入口与模块之前挂上，健康检查不经过这层校验。
func Mount(router *gin.Engine, db *dbx.DB) {
	mountHealth(router)
	ApplyAccess(router, db)
	for _, init := range entries {
		init(router, db)
	}
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		item := modules[name]
		logger.Info(context.Background(), "登记模块 %s %s", item.Name, item.BasePath)
		if item.Init != nil {
			item.Init(router, db)
		}
	}
}
