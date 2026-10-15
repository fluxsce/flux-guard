// Command server 启动 flux-guard 管理端。
// 先按依赖打开配置、日志、库和各服务，再监听 HTTP。
// 退出时先取消运行上下文，等预警扫描和 HTTP 都结束，再关库和日志。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fluxguard/common/buildinfo"
	"fluxguard/common/logger"
	"fluxguard/server/alert"
	"fluxguard/server/command"
	"fluxguard/server/component/bridge"
	"fluxguard/server/config"
	"fluxguard/server/job/policy"
	"fluxguard/server/job/registry"
	"fluxguard/server/job/sched"
	"fluxguard/server/job/status"
	"fluxguard/server/job/tenant"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/plugin"
	"fluxguard/server/retention"
	"fluxguard/server/setting"
	"fluxguard/server/shell"
	"fluxguard/server/store"
	webinit "fluxguard/server/web/init"
	"fluxguard/server/web/request"
	"fluxguard/server/web/routes"
	login "fluxguard/server/web/views/login/controllers"
)

// main 拉起进程。-version 只打印版本。其余失败在资源关闭之后再退出。
func main() {
	buildinfo.ExitIfRequested()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run 打开资源并监听。返回前 defer 逆序执行：先关库，再关日志。
// log.Fatal 会跳过 defer，所以打开失败从这里返回，由 main 再退出。
//
// 初始化顺序：配置和日志、库、环境设置、认证与插件菜单、命令通道、预警菜单、HTTP。
// 认证、命令、上报、预警共用这一条库连接，没有各自的 Close。
//
// 停止顺序：中断、SIGTERM 或监听失败取消运行上下文。
// 预警扫描、终端回收和库表保留清理带着这个上下文，取消后停止新一轮。
// HTTP Shutdown 停接新连接并等在途请求；超时则强制断开。
// 后台 goroutine 都结束后才关库。
func run() error {
	cfg, err := config.Load(os.Getenv("FLUX_GUARD_CONFIG"))
	if err != nil {
		return err
	}
	logs, err := logger.Setup(cfg.Log)
	if err != nil {
		return err
	}
	// 最后关闭，保证前面关库时的错误还能写进日志。
	defer logs.Close()

	driver, dsn, err := store.Resolve(cfg.DB)
	if err != nil {
		return err
	}
	db, err := store.Open(context.Background(), driver, dsn)
	if err != nil {
		return err
	}
	// HTTP 停稳之后关闭。各服务都拿着这条连接。
	defer db.Close()

	if err := setting.Load(context.Background(), db); err != nil {
		return err
	}
	if err := policy.Load(context.Background(), db); err != nil {
		return err
	}
	if err := status.Open(context.Background(), db); err != nil {
		return err
	}
	registry.Register(retention.NewJob(db))
	if err := registry.Restore(context.Background(), db); err != nil {
		return err
	}
	login.Open(db, cfg.TenantID)
	if _, err := command.Open(db); err != nil {
		return err
	}
	alertSvc := alert.Open(db)
	// 扫描循环要等 HTTP 装配完再启动。菜单和授权只在 scripts/db 的种子里。
	handler := webinit.New(db)
	// 有构建产物才挂控制台页面，接口本身不依赖它。
	if info, statErr := os.Stat("web/dist"); statErr == nil && info.IsDir() {
		handler = webinit.Static("web/dist", handler)
	}
	apiSec := request.Timeout(cfg.TenantID).RequestTimeoutSeconds
	request.ConfigureSessionCookie(cfg.SessionCookieSecure())
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: limits.HTTPReadHeader,
		// Read 固定服务大包上传；Write 跟接口超时走，并盖住长轮询 hold。见 server/utils/const/limits。
		ReadTimeout:  limits.HTTPRead,
		WriteTimeout: limits.ServerWrite(apiSec),
		IdleTimeout:  limits.HTTPIdle,
	}
	tlsCfg, err := cfg.TLSConfig()
	if err != nil {
		return err
	}
	srv.TLSConfig = tlsCfg
	// 访问策略变更只调写超时，不改读超时，避免上传与接口预算互相覆盖。
	setting.RegisterHTTPTimeoutApplier(func(apiTimeoutSec int) {
		srv.WriteTimeout = limits.ServerWrite(apiTimeoutSec)
	})

	// 运行上下文。取消它既通知预警扫描退出，也让下面的等待结束。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		alertSvc.Loop(ctx)
	}()
	// 插件先按清单拉起。组件侧再按调用结果换掉对应种类的内置探针。须在调度开始拨号之前。
	if err := plugin.Load(plugin.DirName); err != nil {
		return err
	}
	if err := bridge.Attach(); err != nil {
		plugin.StopAll()
		return err
	}
	loopsDone := routes.RunLoops(ctx)
	shellDone := make(chan struct{})
	go func() {
		defer close(shellDone)
		if s := shell.Global(); s != nil {
			s.Reap(ctx)
		} else {
			shell.OpenStore(db).Reap(ctx)
		}
	}()
	retentionDone := make(chan struct{})
	go func() {
		defer close(retentionDone)
		sched.New(tenant.Lister(db, cfg.TenantID), cfg.TenantID).Start(ctx)
	}()
	listenDone := make(chan struct{})
	go func() {
		defer close(listenDone)
		scheme := "http"
		if cfg.EnableHTTPS {
			scheme = "https"
		}
		logger.Info(ctx, "flux-guard server %s 监听 %s://%s，库 %s", buildinfo.Version, scheme, cfg.Listen, store.Describe(driver, dsn))
		logger.Info(ctx, "开发账号 admin / viewer，口令见 README")
		var err error
		if cfg.EnableHTTPS {
			// 证书已在 TLSConfig；空路径走 ServeTLS 的已装载证书。
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(ctx, "监听: %v", err)
			stop()
		}
	}()

	// 阻塞到收到退出信号，或监听失败主动取消。
	<-ctx.Done()
	plugin.StopAll()
	// 停接新连接，等在途请求。超时则强制断开，避免处理函数占着库连接。
	shut, cancel := context.WithTimeout(context.Background(), limits.ShutdownWait)
	defer cancel()
	if err := srv.Shutdown(shut); err != nil {
		logger.Error(shut, "停止: %v", err)
		_ = srv.Close()
	}
	if err := waitDone(listenDone, limits.ShutdownWait); err != nil {
		logger.Error(ctx, "监听退出: %v", err)
	}
	if err := waitDone(scanDone, limits.ShutdownWait); err != nil {
		logger.Error(ctx, "预警扫描退出: %v", err)
	}
	if err := waitDone(loopsDone, limits.ShutdownWait); err != nil {
		logger.Error(ctx, "功能检查退出: %v", err)
	}
	if err := waitDone(shellDone, limits.ShutdownWait); err != nil {
		logger.Error(ctx, "终端清理退出: %v", err)
	}
	if err := waitDone(retentionDone, limits.ShutdownWait); err != nil {
		logger.Error(ctx, "日志清理退出: %v", err)
	}
	return nil
}

// waitDone 等到 ch 关闭。到时仍未关闭则返回错误，调用方继续释放其余资源。
func waitDone(ch <-chan struct{}, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ch:
		return nil
	case <-timer.C:
		return errors.New("超时")
	}
}
