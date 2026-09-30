// Package id 生成不依赖数据库自增的业务主键。
// server 和各个 Go 代理都使用它。口令、令牌摘要不放在这里。
package id

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"sync/atomic"
	"time"
)

var (
	nodeIPv4 uint32
	procMark uint32
	counter  uint64
)

func init() {
	nodeIPv4 = detectNode()
	var rnd [4]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		binary.BigEndian.PutUint32(rnd[:], uint32(time.Now().UnixNano()))
	}
	procMark = binary.BigEndian.Uint32(rnd[:]) ^ uint32(os.Getpid())
	// 从当前微秒起步，之后只加不减。时钟回拨不会把序号拉回去。
	atomic.StoreUint64(&counter, uint64(time.Now().UnixMicro()))
}

// New 返回 32 位十六进制字符串，可放进 VARCHAR(32)。
// 组成对齐 gateway 的主键：时间起点、节点 IPv4、进程标记、进程内递增序号。
// 同一进程内序号只增不减；不同机器靠 IPv4 区分，同一台机器上的不同进程靠进程标记区分。
func New() string {
	n := atomic.AddUint64(&counter, 1)
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[0:8], n)
	binary.BigEndian.PutUint32(buf[8:12], nodeIPv4)
	binary.BigEndian.PutUint32(buf[12:16], procMark)
	return hex.EncodeToString(buf[:])
}

func detectNode() uint32 {
	if ip := pickIPv4(); ip != nil {
		return binary.BigEndian.Uint32(ip)
	}
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		binary.BigEndian.PutUint32(buf[:], uint32(os.Getpid())^uint32(time.Now().UnixNano()))
	}
	return binary.BigEndian.Uint32(buf[:])
}

func pickIPv4() net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var best net.IP
	bestRank := 99
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, aerr := iface.Addrs()
		if aerr != nil {
			continue
		}
		for _, addr := range addrs {
			ip4 := ipv4Of(addr)
			if ip4 == nil || ip4.IsLoopback() || ip4.IsLinkLocalUnicast() {
				continue
			}
			rank := rankIPv4(ip4)
			if rank < bestRank || (rank == bestRank && ip4.String() < best.String()) {
				best = append(net.IP(nil), ip4...)
				bestRank = rank
			}
		}
	}
	return best
}

func ipv4Of(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP.To4()
	case *net.IPAddr:
		return v.IP.To4()
	default:
		return nil
	}
}

// rankIPv4 越小越优先。192.168 优先于 10，再优先于 172.16/12 上的虚拟网卡。
func rankIPv4(ip net.IP) int {
	if ip[0] == 192 && ip[1] == 168 {
		return 0
	}
	if ip[0] == 10 {
		return 1
	}
	if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
		return 3
	}
	if ip.IsPrivate() {
		return 2
	}
	if ip.IsGlobalUnicast() {
		return 4
	}
	return 9
}
