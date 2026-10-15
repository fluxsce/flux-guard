// Package appside 是应用代理：一台机器一个进程，配置里列出各套运行时。
//
// 目录：
//
//	appside/          门面（Load / Collect / 文件通道），对外 API 稳定
//	appside/core/     公共类型、Mode 登记、备份换包、启停探测工具
//	appside/modes/*   各 mode 实现（tomcat / files / node / jar / static / script）
//
// 新增 mode：在 modes/<name> 实现 core.Mode，init 里 core.RegisterMode，并在 register.go 加空白导入。
package appside
