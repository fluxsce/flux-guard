package host

import (
	"bufio"
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/net"

	"fluxguard/protocol"
)

// socketSnap 是一台机器当前的传输层连接，和网卡流量分开。
type socketSnap struct {
	established uint64
	listen      uint64
	timeWait    uint64
	closeWait   uint64
	synSent     uint64
	synRecv     uint64
	finWait     uint64
	lastAck     uint64
	closing     uint64
	tcpInUse    uint64
	udpInUse    uint64
	retrans     uint64
	udpOK       bool
	hasRetrans  bool
	ok          bool
}

func tcpSamples(ctx context.Context, now time.Time) []protocol.Sample {
	snap := readSockets(ctx)
	if !snap.ok {
		return tcpFromProto(ctx, now)
	}
	base := map[string]string{"source": "host"}
	out := []protocol.Sample{
		point("host.net.tcp.established", float64(snap.established), base, now),
		point("host.net.tcp.listen", float64(snap.listen), base, now),
		point("host.net.tcp.time_wait", float64(snap.timeWait), base, now),
		point("host.net.tcp.close_wait", float64(snap.closeWait), base, now),
		point("host.net.tcp.syn_sent", float64(snap.synSent), base, now),
		point("host.net.tcp.syn_recv", float64(snap.synRecv), base, now),
		point("host.net.tcp.fin_wait", float64(snap.finWait), base, now),
		point("host.net.tcp.last_ack", float64(snap.lastAck), base, now),
		point("host.net.tcp.closing", float64(snap.closing), base, now),
		point("host.net.tcp.inuse", float64(snap.tcpInUse), base, now),
	}
	if snap.udpOK {
		out = append(out, point("host.net.udp.inuse", float64(snap.udpInUse), base, now))
	}
	if snap.hasRetrans {
		out = append(out, point("host.net.tcp.retransmits", float64(snap.retrans), base, now))
	} else {
		out = append(out, tcpRetrans(ctx, now)...)
	}
	return out
}

func tcpFromProto(ctx context.Context, now time.Time) []protocol.Sample {
	base := map[string]string{"source": "host"}
	var out []protocol.Sample
	counters, err := net.ProtoCountersWithContext(ctx, []string{"tcp"})
	if err != nil {
		return nil
	}
	for _, item := range counters {
		if strings.ToLower(item.Protocol) != "tcp" {
			continue
		}
		if value, ok := item.Stats["CurrEstab"]; ok {
			out = append(out, point("host.net.tcp.established", float64(value), base, now))
		}
		if value, ok := item.Stats["RetransSegs"]; ok {
			out = append(out, point("host.net.tcp.retransmits", float64(value), base, now))
		}
	}
	return out
}

func tcpRetrans(ctx context.Context, now time.Time) []protocol.Sample {
	base := map[string]string{"source": "host"}
	counters, err := net.ProtoCountersWithContext(ctx, []string{"tcp"})
	if err != nil {
		return nil
	}
	var out []protocol.Sample
	for _, item := range counters {
		if strings.ToLower(item.Protocol) != "tcp" {
			continue
		}
		if value, ok := item.Stats["RetransSegs"]; ok {
			out = append(out, point("host.net.tcp.retransmits", float64(value), base, now))
		}
	}
	return out
}

func addTCPState(snap *socketSnap, state string) {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "01", "ESTABLISHED":
		snap.established++
	case "02", "SYN_SENT":
		snap.synSent++
	case "03", "SYN_RECV", "SYN_RECEIVED":
		snap.synRecv++
	case "04", "FIN_WAIT1", "05", "FIN_WAIT2":
		snap.finWait++
	case "06", "TIME_WAIT":
		snap.timeWait++
	case "08", "CLOSE_WAIT":
		snap.closeWait++
	case "09", "LAST_ACK":
		snap.lastAck++
	case "0A", "LISTEN":
		snap.listen++
	case "0B", "CLOSING":
		snap.closing++
	}
}

// countTCPTable 统计 /proc/net/tcp 风格的文本。第一行是表头。
func countTCPTable(text string, snap *socketSnap) {
	sc := bufio.NewScanner(strings.NewReader(text))
	header := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if header {
			header = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		addTCPState(snap, fields[3])
		snap.ok = true
	}
}

func parseSockstat(text string, snap *socketSnap) {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		kind := strings.TrimSuffix(fields[0], ":")
		vals := map[string]uint64{}
		for i := 1; i+1 < len(fields); i += 2 {
			n, err := strconv.ParseUint(fields[i+1], 10, 64)
			if err != nil {
				continue
			}
			vals[fields[i]] = n
		}
		switch kind {
		case "TCP", "TCP6":
			snap.tcpInUse += vals["inuse"]
			snap.ok = true
		case "UDP", "UDP6":
			snap.udpInUse += vals["inuse"]
			snap.udpOK = true
			snap.ok = true
		}
	}
}

func parseTCPRetrans(text string) (uint64, bool) {
	var header []string
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "Tcp:" {
			continue
		}
		if header == nil {
			header = fields
			continue
		}
		idx := -1
		for i, name := range header {
			if name == "RetransSegs" {
				idx = i
				break
			}
		}
		if idx < 0 || idx >= len(fields) {
			return 0, false
		}
		n, err := strconv.ParseUint(fields[idx], 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

func fillTCPInUse(snap *socketSnap) {
	if snap.tcpInUse > 0 {
		return
	}
	snap.tcpInUse = snap.established + snap.listen + snap.timeWait + snap.closeWait +
		snap.synSent + snap.synRecv + snap.finWait + snap.lastAck + snap.closing
}
