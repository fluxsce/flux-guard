// Package job 是进程内的系统任务管理器。
//
// 业务实现 contract.Job，进程启动时 registry.Register。前端触发和定时触发都走 exec.Run：
// 读取 policy 里的调度信封，把 params 原样传入 Run，再把成功失败、耗时和 Result.Data 写入 status。
//
//	contract  任务接口、触发来源、调用参数和通用结果
//	registry  已注册、可执行的任务
//	catalog   控制台目录。目录里有、registry 里没有的项不会被调度
//	policy    是否启用，手动、按间隔、指定时间或 cron，以及超时和重试；params 由业务解释
//	exec      一次调用，并回写状态与执行日志
//	sched     单进程按固定节拍轮询到期任务
//	status    FG_JOB 保存最近一次，FG_JOB_LOG 追加每一次
//	tenant    给租户级任务提供租户列表
//	ext       具体任务的策略。数据归档在 ext/archive，执行在 retention
//
// 调度只在当前进程内进行，节拍固定。同一租户的同一任务同时只跑一次。
// 单次调用有超时，失败按策略重试。调度方式为手动、固定间隔、指定时间一次，或任意五段 cron。
// 多实例互斥和任务依赖不在本包。
package job
