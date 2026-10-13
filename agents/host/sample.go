package host

import (
	"context"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"

	"fluxguard/common/hostnic"
	"fluxguard/common/hostvol"
	"fluxguard/protocol"
)

const (
	maxCPU      = 128
	maxDisk     = 32
	maxNIC      = 32
	maxProc     = 10
	maxLabelLen = 128
)

// 伪文件系统不代表一块可容量磁盘。
var skipFS = map[string]bool{
	"tmpfs": true, "devtmpfs": true, "overlay": true, "squashfs": true,
	"autofs": true, "proc": true, "sysfs": true, "cgroup": true, "cgroup2": true,
	"devfs": true, "iso9660": true, "cdfs": true, "tracefs": true, "debugfs": true,
	"nsfs": true, "ramfs": true, "bpf": true,
}

type ioSnap struct {
	readBytes, writeBytes uint64
	readCount, writeCount uint64
	readTime, writeTime   uint64
	ioTime                uint64
}

type procCPU struct {
	user, system          float64
	readBytes, writeBytes uint64
	ppid                  int32
	hasCPU, hasIO         bool
}

type nicSnap struct {
	rxBytes, txBytes     uint64
	rxPackets, txPackets uint64
	rxErrors, txErrors   uint64
	rxDrop, txDrop       uint64
}

type history struct {
	at   time.Time
	cpu  cpu.TimesStat
	cpus map[string]cpu.TimesStat
	disk map[string]ioSnap
	nic  map[string]nicSnap
	proc map[int32]procCPU
}

var (
	histMu sync.Mutex
	hist   *history
)

func sample(ctx context.Context, now time.Time) ([]protocol.Sample, uint64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	histMu.Lock()
	prev := hist
	histMu.Unlock()
	cur := &history{
		at: now, cpus: map[string]cpu.TimesStat{}, disk: map[string]ioSnap{},
		nic: map[string]nicSnap{}, proc: map[int32]procCPU{},
	}
	var out []protocol.Sample
	out = append(out, cpuSamples(ctx, now, prev, cur)...)
	out = append(out, memSamples(ctx, vm, now)...)
	out = append(out, diskSamples(ctx, now)...)
	out = append(out, diskIOSamples(ctx, now, prev, cur)...)
	out = append(out, nicSamples(ctx, now, prev, cur)...)
	out = append(out, procSamples(ctx, now, prev, cur)...)
	out = append(out, tcpSamples(ctx, now)...)
	out = append(out, fdSamples(now)...)
	out = append(out, uptimeSample(now)...)
	histMu.Lock()
	hist = cur
	histMu.Unlock()
	return out, vm.Total, nil
}

func cpuSamples(ctx context.Context, now time.Time, prev, cur *history) []protocol.Sample {
	logical, _ := cpu.CountsWithContext(ctx, true)
	physical, _ := cpu.CountsWithContext(ctx, false)
	base := map[string]string{"source": "host"}
	out := []protocol.Sample{
		point("host.cpu.count", float64(logical), base, now),
		point("host.cpu.core.count", float64(physical), base, now),
	}
	times, err := cpu.TimesWithContext(ctx, false)
	per, perErr := cpu.TimesWithContext(ctx, true)
	if err == nil && len(times) > 0 {
		cur.cpu = times[0]
		if prev != nil {
			if usage, user, system, idle, iowait, irq, soft, steal, ok := cpuSplit(times[0], prev.cpu); ok {
				out = append(out,
					point("host.cpu.usage.percent", usage, base, now),
					point("host.cpu.user.percent", user, base, now),
					point("host.cpu.system.percent", system, base, now),
					point("host.cpu.idle.percent", idle, base, now),
					point("host.cpu.iowait.percent", iowait, base, now),
					point("host.cpu.irq.percent", irq, base, now),
					point("host.cpu.softirq.percent", soft, base, now),
					point("host.cpu.steal.percent", steal, base, now),
				)
			}
		}
	}
	if perErr == nil {
		if len(per) > maxCPU {
			per = per[:maxCPU]
		}
		for _, item := range per {
			cur.cpus[item.CPU] = item
			if prev == nil {
				continue
			}
			old, ok := prev.cpus[item.CPU]
			if !ok {
				continue
			}
			if usage, _, _, _, _, _, _, _, ok := cpuSplit(item, old); ok {
				out = append(out, point("host.cpu.core.usage.percent", usage, labels("cpu", item.CPU), now))
			}
		}
	}
	if avg, err := load.AvgWithContext(ctx); err == nil {
		out = append(out,
			point("host.cpu.load1", avg.Load1, base, now),
			point("host.cpu.load5", avg.Load5, base, now),
			point("host.cpu.load15", avg.Load15, base, now),
		)
	}
	return out
}

func memSamples(ctx context.Context, vm *mem.VirtualMemoryStat, now time.Time) []protocol.Sample {
	base := map[string]string{"source": "host"}
	out := []protocol.Sample{
		point("host.mem.total.bytes", float64(vm.Total), base, now),
		point("host.mem.available.bytes", float64(vm.Available), base, now),
		point("host.mem.used.bytes", float64(vm.Used), base, now),
		point("host.mem.free.bytes", float64(vm.Free), base, now),
		point("host.mem.used.percent", vm.UsedPercent, base, now),
		point("host.mem.cached.bytes", float64(vm.Cached), base, now),
		point("host.mem.buffers.bytes", float64(vm.Buffers), base, now),
	}
	if swap, err := mem.SwapMemoryWithContext(ctx); err == nil {
		out = append(out,
			point("host.mem.swap.total.bytes", float64(swap.Total), base, now),
			point("host.mem.swap.used.bytes", float64(swap.Used), base, now),
			point("host.mem.swap.free.bytes", float64(swap.Free), base, now),
			point("host.mem.swap.used.percent", swap.UsedPercent, base, now),
		)
	}
	return out
}

func diskSamples(ctx context.Context, now time.Time) []protocol.Sample {
	parts, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil
	}
	var out []protocol.Sample
	n := 0
	seen := map[string]bool{}
	for _, part := range parts {
		if n >= maxDisk || skipFS[strings.ToLower(part.Fstype)] || part.Mountpoint == "" || hostvol.Container(part.Mountpoint) {
			continue
		}
		if seen[part.Mountpoint] {
			continue
		}
		usage, err := disk.UsageWithContext(ctx, part.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}
		seen[part.Mountpoint] = true
		n++
		lab := labels("path", part.Mountpoint, "device", part.Device)
		out = append(out,
			point("host.disk.total.bytes", float64(usage.Total), lab, now),
			point("host.disk.free.bytes", float64(usage.Free), lab, now),
			point("host.disk.used.bytes", float64(usage.Used), lab, now),
			point("host.disk.used.percent", usage.UsedPercent, lab, now),
		)
		if usage.InodesTotal > 0 {
			out = append(out,
				point("host.disk.inodes.total", float64(usage.InodesTotal), lab, now),
				point("host.disk.inodes.used", float64(usage.InodesUsed), lab, now),
				point("host.disk.inodes.free", float64(usage.InodesFree), lab, now),
				point("host.disk.inodes.used.percent", usage.InodesUsedPercent, lab, now),
			)
		}
	}
	return out
}

func diskIOSamples(ctx context.Context, now time.Time, prev, cur *history) []protocol.Sample {
	counters, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(counters))
	for name := range counters {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > maxDisk {
		names = names[:maxDisk]
	}
	var out []protocol.Sample
	var dt time.Duration
	if prev != nil {
		dt = now.Sub(prev.at)
	}
	for _, name := range names {
		item := counters[name]
		snap := ioSnap{
			readBytes: item.ReadBytes, writeBytes: item.WriteBytes,
			readCount: item.ReadCount, writeCount: item.WriteCount,
			readTime: item.ReadTime, writeTime: item.WriteTime, ioTime: item.IoTime,
		}
		cur.disk[name] = snap
		lab := labels("device", name)
		out = append(out,
			point("host.disk.io.read.bytes", float64(item.ReadBytes), lab, now),
			point("host.disk.io.write.bytes", float64(item.WriteBytes), lab, now),
			point("host.disk.io.reads", float64(item.ReadCount), lab, now),
			point("host.disk.io.writes", float64(item.WriteCount), lab, now),
			point("host.disk.io.read.time_ms", float64(item.ReadTime), lab, now),
			point("host.disk.io.write.time_ms", float64(item.WriteTime), lab, now),
		)
		if prev == nil || dt <= 0 {
			continue
		}
		old, ok := prev.disk[name]
		if !ok {
			continue
		}
		out = append(out,
			point("host.disk.io.read.bytes_per_sec", perSec(snap.readBytes, old.readBytes, dt), lab, now),
			point("host.disk.io.write.bytes_per_sec", perSec(snap.writeBytes, old.writeBytes, dt), lab, now),
		)
		if latency, ok := latencyMs(snap.readCount, old.readCount, snap.readTime, old.readTime); ok {
			out = append(out, point("host.disk.io.read.latency_ms", latency, lab, now))
		}
		if latency, ok := latencyMs(snap.writeCount, old.writeCount, snap.writeTime, old.writeTime); ok {
			out = append(out, point("host.disk.io.write.latency_ms", latency, lab, now))
		}
		if util, ok := ioUtil(snap.ioTime, old.ioTime, dt); ok {
			out = append(out, point("host.disk.io.util.percent", util, lab, now))
		}
	}
	return out
}

func nicSamples(ctx context.Context, now time.Time, prev, cur *history) []protocol.Sample {
	counters, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil
	}
	var out []protocol.Sample
	var dt time.Duration
	if prev != nil {
		dt = now.Sub(prev.at)
	}
	n := 0
	for _, item := range counters {
		if n >= maxNIC || skipNIC(item.Name) {
			continue
		}
		n++
		snap := nicSnap{
			rxBytes: item.BytesRecv, txBytes: item.BytesSent,
			rxPackets: item.PacketsRecv, txPackets: item.PacketsSent,
			rxErrors: item.Errin, txErrors: item.Errout,
			rxDrop: item.Dropin, txDrop: item.Dropout,
		}
		cur.nic[item.Name] = snap
		lab := labels("device", item.Name)
		out = append(out,
			point("host.net.receive.bytes", float64(snap.rxBytes), lab, now),
			point("host.net.transmit.bytes", float64(snap.txBytes), lab, now),
			point("host.net.receive.packets", float64(snap.rxPackets), lab, now),
			point("host.net.transmit.packets", float64(snap.txPackets), lab, now),
			point("host.net.receive.errs", float64(snap.rxErrors), lab, now),
			point("host.net.transmit.errs", float64(snap.txErrors), lab, now),
			point("host.net.receive.drop", float64(snap.rxDrop), lab, now),
			point("host.net.transmit.drop", float64(snap.txDrop), lab, now),
		)
		if prev == nil || dt <= 0 {
			continue
		}
		old, ok := prev.nic[item.Name]
		if !ok {
			continue
		}
		out = append(out,
			point("host.net.receive.bytes_per_sec", perSec(snap.rxBytes, old.rxBytes, dt), lab, now),
			point("host.net.transmit.bytes_per_sec", perSec(snap.txBytes, old.txBytes, dt), lab, now),
		)
	}
	return out
}

type procMem struct {
	name              string
	pid               int32
	rss               uint64
	cpu               float64
	readBps, writeBps float64
	has, hasIO        bool
}

func procSamples(ctx context.Context, now time.Time, prev, cur *history) []protocol.Sample {
	list, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil
	}
	base := map[string]string{"source": "host"}
	var running, sleeping, zombie int
	var totalRSS uint64
	cores, _ := cpu.CountsWithContext(ctx, true)
	var dt time.Duration
	if prev != nil {
		dt = now.Sub(prev.at)
	}
	all := make([]procMem, 0, len(list))
	for _, proc := range list {
		if st, err := proc.StatusWithContext(ctx); err == nil {
			switch procState(st) {
			case "running":
				running++
			case "sleeping":
				sleeping++
			case "zombie":
				zombie++
			}
		}
		info, err := proc.MemoryInfoWithContext(ctx)
		if err != nil || info == nil {
			// 进程还在，只是读不到内存。留在本拍里，避免被当成已退出。
			cur.proc[proc.Pid] = procCPU{}
			continue
		}
		totalRSS += info.RSS
		name, _ := proc.NameWithContext(ctx)
		if name == "" {
			name = "unknown"
		}
		item := procMem{name: name, pid: proc.Pid, rss: info.RSS}
		snap := procCPU{}
		if ppid, err := proc.PpidWithContext(ctx); err == nil {
			snap.ppid = ppid
		}
		if times, err := proc.TimesWithContext(ctx); err == nil && times != nil {
			snap.user = times.User
			snap.system = times.System
			snap.hasCPU = true
			if prev != nil {
				if old, ok := prev.proc[proc.Pid]; ok && old.hasCPU {
					if pct, ok := procPercent(snap, old, dt, cores); ok {
						item.cpu = pct
						item.has = true
					}
				}
			}
		}
		// read_bytes / write_bytes 是落到存储的字节。读不到其他用户的进程时跳过。
		if ioStat, err := proc.IOCountersWithContext(ctx); err == nil && ioStat != nil {
			snap.readBytes = ioStat.ReadBytes
			snap.writeBytes = ioStat.WriteBytes
			snap.hasIO = true
			if prev != nil && dt > 0 {
				if old, ok := prev.proc[proc.Pid]; ok && old.hasIO {
					item.readBps = perSec(snap.readBytes, old.readBytes, dt)
					item.writeBps = perSec(snap.writeBytes, old.writeBytes, dt)
					item.hasIO = true
				}
			}
		}
		cur.proc[proc.Pid] = snap
		all = append(all, item)
	}
	if runtime.GOOS == "linux" {
		adjustLinuxProcIO(all, prev, cur, dt)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].rss > all[j].rss })
	rssTop := all
	if len(rssTop) > maxProc {
		rssTop = rssTop[:maxProc]
	}
	out := []protocol.Sample{
		point("host.proc.count", float64(len(list)), base, now),
		point("host.proc.running.count", float64(running), base, now),
		point("host.proc.sleeping.count", float64(sleeping), base, now),
		point("host.proc.zombie.count", float64(zombie), base, now),
		point("host.proc.rss.total.bytes", float64(totalRSS), base, now),
	}
	for _, item := range rssTop {
		out = append(out, point("host.proc.rss.bytes", float64(item.rss), labels("name", item.name, "pid", itoa(item.pid)), now))
	}
	cpuTop := append([]procMem(nil), all...)
	sort.Slice(cpuTop, func(i, j int) bool { return cpuTop[i].cpu > cpuTop[j].cpu })
	n := 0
	for _, item := range cpuTop {
		if !item.has || n >= maxProc {
			continue
		}
		n++
		out = append(out, point("host.proc.cpu.percent", item.cpu, labels("name", item.name, "pid", itoa(item.pid)), now))
	}
	return append(out, procIOPoints(now, all)...)
}

// procIOPoints 上报这一间隔里读或写靠前的进程。同一进程读和写都带上，表上能对上同一次采样。
func procIOPoints(now time.Time, all []procMem) []protocol.Sample {
	active := make([]procMem, 0, len(all))
	for _, item := range all {
		if suppressProcIO(item) || !item.hasIO || (item.readBps == 0 && item.writeBps == 0) {
			continue
		}
		active = append(active, item)
	}
	if len(active) == 0 {
		return nil
	}
	chosen := map[int32]procMem{}
	take := func(higher func(a, b procMem) bool) {
		sort.Slice(active, func(i, j int) bool { return higher(active[i], active[j]) })
		n := 0
		for _, item := range active {
			if _, ok := chosen[item.pid]; ok {
				continue
			}
			if n >= maxProc {
				return
			}
			chosen[item.pid] = item
			n++
		}
	}
	take(func(a, b procMem) bool { return a.readBps > b.readBps })
	take(func(a, b procMem) bool { return a.writeBps > b.writeBps })
	out := make([]protocol.Sample, 0, len(chosen)*2)
	for _, item := range chosen {
		lab := labels("name", item.name, "pid", itoa(item.pid))
		out = append(out,
			point("host.proc.io.read.bytes_per_sec", item.readBps, lab, now),
			point("host.proc.io.write.bytes_per_sec", item.writeBps, lab, now),
		)
	}
	return out
}

// adjustLinuxProcIO 去掉收尸带进来的历史字节。内核在 wait 时把子进程一辈子的
// read_bytes / write_bytes 加进父进程，直接做差会把整段历史显示成这一拍的速度。
// 上一拍见过的子进程可以减掉。这一拍里才出生并退出的减不掉，1 号进程和 systemd 不再展示。
func adjustLinuxProcIO(all []procMem, prev, cur *history, dt time.Duration) {
	if prev == nil || cur == nil || dt <= 0 {
		for i := range all {
			if linuxReaper(all[i]) {
				all[i].hasIO = false
				all[i].readBps = 0
				all[i].writeBps = 0
			}
		}
		return
	}
	reapedRead := map[int32]uint64{}
	reapedWrite := map[int32]uint64{}
	for pid, old := range prev.proc {
		if _, alive := cur.proc[pid]; alive || !old.hasIO || old.ppid <= 0 {
			continue
		}
		reapedRead[old.ppid] += old.readBytes
		reapedWrite[old.ppid] += old.writeBytes
	}
	sec := dt.Seconds()
	for i := range all {
		if !all[i].hasIO && !linuxReaper(all[i]) {
			continue
		}
		read := all[i].readBps*sec - float64(reapedRead[all[i].pid])
		write := all[i].writeBps*sec - float64(reapedWrite[all[i].pid])
		if read < 0 {
			read = 0
		}
		if write < 0 {
			write = 0
		}
		all[i].readBps = read / sec
		all[i].writeBps = write / sec
		if linuxReaper(all[i]) || (all[i].readBps == 0 && all[i].writeBps == 0) {
			all[i].hasIO = false
			all[i].readBps = 0
			all[i].writeBps = 0
		}
	}
}

func suppressProcIO(item procMem) bool {
	return runtime.GOOS == "linux" && linuxReaper(item)
}

func linuxReaper(item procMem) bool {
	return item.pid == 1 || item.name == "systemd"
}

func cpuSplit(cur, prev cpu.TimesStat) (usage, user, system, idle, iowait, irq, soft, steal float64, ok bool) {
	delta := func(a, b float64) float64 {
		if a < b {
			return 0
		}
		return a - b
	}
	user = delta(cur.User, prev.User)
	system = delta(cur.System, prev.System)
	idle = delta(cur.Idle, prev.Idle)
	iowait = delta(cur.Iowait, prev.Iowait)
	irq = delta(cur.Irq, prev.Irq)
	soft = delta(cur.Softirq, prev.Softirq)
	steal = delta(cur.Steal, prev.Steal)
	nice := delta(cur.Nice, prev.Nice)
	total := user + system + idle + iowait + irq + soft + steal + nice
	if total <= 0 {
		return
	}
	scale := 100 / total
	return (total - idle) * scale, user * scale, system * scale, idle * scale, iowait * scale, irq * scale, soft * scale, steal * scale, true
}

func procState(status []string) string {
	for _, item := range status {
		switch strings.ToLower(item) {
		case "r", "running":
			return "running"
		case "s", "sleep", "sleeping", "idle", "disk sleep", "i":
			return "sleeping"
		case "z", "zombie":
			return "zombie"
		}
	}
	return ""
}

func skipNIC(name string) bool {
	n := strings.ToLower(name)
	return n == "lo" || n == "lo0" || strings.Contains(n, "loopback") || hostnic.Container(n)
}

func fdSamples(now time.Time) []protocol.Sample {
	allocated, maximum, ok := fileDescriptorLimits()
	if !ok {
		return nil
	}
	base := map[string]string{"source": "host"}
	out := []protocol.Sample{
		point("host.fd.allocated", float64(allocated), base, now),
		point("host.fd.max", float64(maximum), base, now),
	}
	if maximum > 0 {
		out = append(out, point("host.fd.used.percent", float64(allocated)/float64(maximum)*100, base, now))
	}
	return out
}

func uptimeSample(now time.Time) []protocol.Sample {
	boot := bootTime()
	if boot.IsZero() {
		return nil
	}
	seconds := now.Sub(boot).Seconds()
	if seconds < 0 {
		return nil
	}
	return []protocol.Sample{point("host.uptime.seconds", seconds, map[string]string{"source": "host"}, now)}
}

func latencyMs(nowCount, prevCount, nowTime, prevTime uint64) (float64, bool) {
	if nowCount <= prevCount || nowTime < prevTime {
		return 0, false
	}
	return float64(nowTime-prevTime) / float64(nowCount-prevCount), true
}

func ioUtil(now, prev uint64, dt time.Duration) (float64, bool) {
	ms := dt.Milliseconds()
	if ms <= 0 || now < prev {
		return 0, false
	}
	pct := float64(now-prev) / float64(ms) * 100
	if pct > 100 {
		pct = 100
	}
	return pct, true
}

func procPercent(cur, prev procCPU, dt time.Duration, cores int) (float64, bool) {
	if dt <= 0 || cores <= 0 {
		return 0, false
	}
	delta := (cur.user - prev.user) + (cur.system - prev.system)
	if delta < 0 {
		return 0, false
	}
	return delta / dt.Seconds() * 100 / float64(cores), true
}

func perSec(now, prev uint64, dt time.Duration) float64 {
	if dt <= 0 || now < prev {
		return 0
	}
	return float64(now-prev) / dt.Seconds()
}

func point(name string, value float64, labels map[string]string, now time.Time) protocol.Sample {
	return protocol.Sample{MetricName: name, MetricValue: value, Labels: labels, ObservedAt: now}
}

func labels(pairs ...string) map[string]string {
	out := map[string]string{"source": "host"}
	for i := 0; i+1 < len(pairs); i += 2 {
		out[pairs[i]] = clip(pairs[i+1])
	}
	return out
}

func clip(value string) string {
	if len(value) <= maxLabelLen {
		return value
	}
	cut := value[:maxLabelLen]
	for !utf8.ValidString(cut) && cut != "" {
		cut = cut[:len(cut)-1]
	}
	return cut
}

func itoa(n int32) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	u := uint32(n)
	neg := n < 0
	if neg {
		u = uint32(-n)
	}
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
