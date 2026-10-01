package kafka

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
)

const lagGroupCap int32 = 500

func collectGroups(ctx context.Context, seed string, brokers []BrokerStat, opt link, topics []TopicStat, apis apiSet) ([]GroupStat, int64, int64, int64, int64, string) {
	names, err := listAllGroups(ctx, seed, brokers, opt, apis)
	if err != nil {
		return nil, 0, -1, -1, 0, "消费组暂时读不到"
	}
	if len(names) == 0 {
		return nil, 0, -1, -1, 0, ""
	}
	groupCount := int64(len(names))
	located := locateGroups(ctx, seed, opt, names, apis)
	byAddr := map[string][]string{}
	var missing []GroupStat
	for _, name := range names {
		addr := located[name]
		if addr == "" {
			missing = append(missing, GroupStat{Name: name, Lag: -1, MaxPart: -1, Part: -1})
			continue
		}
		byAddr[addr] = append(byAddr[addr], name)
	}
	var left int32 = lagGroupCap
	var mu sync.Mutex
	var out []GroupStat
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for addr, groupNames := range byAddr {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(addr string, groupNames []string) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()
			stats := inspectCoordinator(ctx, addr, opt, groupNames, topics, apis, &left)
			mu.Lock()
			out = append(out, stats...)
			mu.Unlock()
		}(addr, groupNames)
	}
	wg.Wait()
	out = append(out, missing...)
	maxLag, totalLag, rebalance, measured := foldGroups(out)
	note := ""
	if int64(measured) < groupCount {
		note = fmt.Sprintf("消费组共 %d 个，这次读完了 %d 个的延迟", groupCount, measured)
	}
	return out, groupCount, maxLag, totalLag, rebalance, note
}

func foldGroups(groups []GroupStat) (maxLag, totalLag, rebalance int64, measured int) {
	maxLag = -1
	var known bool
	for _, item := range groups {
		switch item.State {
		case "PreparingRebalance", "CompletingRebalance":
			rebalance++
		}
		if item.MaxPart < 0 {
			continue
		}
		known = true
		measured++
		if item.MaxPart > maxLag {
			maxLag = item.MaxPart
		}
		if item.Lag > 0 {
			totalLag += item.Lag
		}
	}
	if !known {
		return -1, -1, rebalance, 0
	}
	return maxLag, totalLag, rebalance, measured
}

func listAllGroups(ctx context.Context, seed string, brokers []BrokerStat, opt link, apis apiSet) ([]string, error) {
	addrs := []string{seed}
	seen := map[string]struct{}{seed: {}}
	for _, item := range brokers {
		if !item.Up {
			continue
		}
		addr := brokerAddr(item)
		if addr == "" || addr == ":" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		addrs = append(addrs, addr)
	}
	var mu sync.Mutex
	names := map[string]struct{}{}
	var anyOK bool
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, addr := range addrs {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()
			got, err := listGroupsAt(ctx, addr, opt, apis)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				return
			}
			anyOK = true
			for _, name := range got {
				names[name] = struct{}{}
			}
		}(addr)
	}
	wg.Wait()
	if !anyOK {
		return nil, fmt.Errorf("消费组暂时读不到")
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func listGroupsAt(ctx context.Context, addr string, opt link, apis apiSet) ([]string, error) {
	raw, err := openAuth(ctx, addr, opt)
	if err != nil {
		return nil, err
	}
	defer raw.Close()
	ver, ok := apis.use(16, 0)
	if !ok {
		ver, ok = apis.use(16, 1)
		if !ok {
			return nil, fmt.Errorf("Kafka 消费组列表失败")
		}
	}
	body, err := roundTrip(raw, 16, ver, 20, nil)
	if err != nil {
		return nil, err
	}
	return parseListGroups(body, ver)
}

func openAuth(ctx context.Context, addr string, opt link) (net.Conn, error) {
	raw, _, err := connect(ctx, addr, opt)
	return raw, err
}

func locateGroups(ctx context.Context, seed string, opt link, names []string, apis apiSet) map[string]string {
	out := map[string]string{}
	if len(names) == 0 {
		return out
	}
	jobs := make(chan string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	workers := 4
	if len(names) < workers {
		workers = len(names)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			raw, err := openAuth(ctx, seed, opt)
			if err != nil {
				for range jobs {
				}
				return
			}
			defer raw.Close()
			ver, ok := apis.use(10, 0)
			if !ok {
				ver, ok = apis.use(10, 1)
				if !ok {
					for range jobs {
					}
					return
				}
			}
			corr := int32(30)
			for name := range jobs {
				if ctx.Err() != nil {
					continue
				}
				body, err := roundTrip(raw, 10, ver, corr, findCoordinatorBody(name, ver))
				corr++
				if err != nil {
					continue
				}
				host, port, err := parseCoordinator(body, ver)
				if err != nil || host == "" || port <= 0 {
					continue
				}
				mu.Lock()
				out[name] = net.JoinHostPort(host, strconv.Itoa(int(port)))
				mu.Unlock()
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, name := range names {
			select {
			case <-ctx.Done():
				return
			case jobs <- name:
			}
		}
	}()
	wg.Wait()
	return out
}

func inspectCoordinator(ctx context.Context, addr string, opt link, names []string, topics []TopicStat, apis apiSet, left *int32) []GroupStat {
	raw, err := openAuth(ctx, addr, opt)
	if err != nil {
		return unknownGroups(names)
	}
	defer raw.Close()
	infos := describeAll(raw, names, apis)
	ordered := append([]string(nil), names...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return stateRank(infos[ordered[i]].State) < stateRank(infos[ordered[j]].State)
	})
	ofVer, ofOK := offsetFetchVersion(apis)
	commits := map[string]map[string]map[int32]int64{}
	out := make([]GroupStat, 0, len(ordered))
	corr := int32(40)
	for _, name := range ordered {
		info := infos[name]
		item := GroupStat{Name: name, Lag: -1, MaxPart: -1, Part: -1, State: info.State, Members: info.Members}
		if !ofOK || !takeLag(left) || ctx.Err() != nil {
			out = append(out, item)
			continue
		}
		body, err := roundTrip(raw, 9, ofVer, corr, offsetFetchBody(ofVer, name, topics))
		corr++
		if err != nil {
			out = append(out, item)
			continue
		}
		parsed, err := parseOffsetFetch(body, ofVer)
		if err != nil {
			out = append(out, item)
			continue
		}
		commits[name] = parsed
		out = append(out, item)
	}
	ends := fetchEnds(raw, commits, apis, &corr)
	for i := range out {
		total, maxPart, topic, part, nTopic, ok := summarizeLag(commits[out[i].Name], ends)
		if !ok {
			continue
		}
		out[i].Lag = total
		out[i].MaxPart = maxPart
		out[i].Part = part
		out[i].Topic = topic
		out[i].Topics = nTopic
	}
	return out
}

func unknownGroups(names []string) []GroupStat {
	out := make([]GroupStat, len(names))
	for i, name := range names {
		out[i] = GroupStat{Name: name, Lag: -1, MaxPart: -1, Part: -1}
	}
	return out
}

func describeAll(raw net.Conn, names []string, apis apiSet) map[string]groupInfo {
	out := map[string]groupInfo{}
	ver, ok := apis.use(15, 0)
	if !ok {
		ver, ok = apis.use(15, 1)
		if !ok {
			return out
		}
	}
	corr := int32(60)
	for _, chunk := range chunks(names, 25) {
		body, err := roundTrip(raw, 15, ver, corr, describeGroupsBody(chunk))
		corr++
		if err != nil {
			continue
		}
		parsed, err := ParseDescribeGroups(body, ver)
		if err != nil {
			continue
		}
		for name, info := range parsed {
			out[name] = info
		}
	}
	return out
}

func fetchEnds(raw net.Conn, commits map[string]map[string]map[int32]int64, apis apiSet, corr *int32) map[string]map[int32]int64 {
	seen := map[string]map[int32]struct{}{}
	var flat []partRef
	for _, topics := range commits {
		for topic, parts := range topics {
			for part := range parts {
				if seen[topic] == nil {
					seen[topic] = map[int32]struct{}{}
				}
				if _, ok := seen[topic][part]; ok {
					continue
				}
				seen[topic][part] = struct{}{}
				flat = append(flat, partRef{topic: topic, part: part})
			}
		}
	}
	ver, ok := apis.use(2, 1)
	if !ok {
		ver, ok = apis.use(2, 2)
		if !ok {
			return nil
		}
	}
	out := map[string]map[int32]int64{}
	for start := 0; start < len(flat); start += 400 {
		end := start + 400
		if end > len(flat) {
			end = len(flat)
		}
		chunk := map[string][]int32{}
		for _, item := range flat[start:end] {
			chunk[item.topic] = append(chunk[item.topic], item.part)
		}
		*corr++
		body, err := roundTrip(raw, 2, ver, *corr, listOffsetsBody(ver, chunk))
		if err != nil {
			continue
		}
		parsed, err := parseListOffsets(body, ver)
		if err != nil {
			continue
		}
		for topic, parts := range parsed {
			if out[topic] == nil {
				out[topic] = map[int32]int64{}
			}
			for part, offset := range parts {
				out[topic][part] = offset
			}
		}
	}
	return out
}

type partRef struct {
	topic string
	part  int32
}

func takeLag(left *int32) bool {
	for {
		cur := atomic.LoadInt32(left)
		if cur <= 0 {
			return false
		}
		if atomic.CompareAndSwapInt32(left, cur, cur-1) {
			return true
		}
	}
}

func stateRank(state string) int {
	switch state {
	case "Dead", "PreparingRebalance", "CompletingRebalance":
		return 0
	case "Empty":
		return 1
	default:
		return 2
	}
}

func offsetFetchVersion(apis apiSet) (int16, bool) {
	if ver, ok := apis.use(9, 3); ok {
		return ver, true
	}
	return 0, false
}

func findCoordinatorBody(name string, ver int16) []byte {
	body := kafkaString(name)
	if ver >= 1 {
		body = append(body, 0)
	}
	return body
}

func describeGroupsBody(names []string) []byte {
	body := appendInt32(nil, int32(len(names)))
	for _, name := range names {
		body = append(body, kafkaString(name)...)
	}
	return body
}

func offsetFetchBody(ver int16, group string, topics []TopicStat) []byte {
	if ver >= 2 {
		return append(kafkaString(group), 0xff, 0xff, 0xff, 0xff)
	}
	return offsetFetchExplicit(group, topics)
}

func offsetFetchExplicit(group string, topics []TopicStat) []byte {
	left := 512
	var picked []TopicStat
	for _, topic := range topics {
		if left <= 0 {
			break
		}
		parts := topic.partIDs
		if len(parts) > left {
			parts = parts[:left]
		}
		left -= len(parts)
		topic.partIDs = parts
		picked = append(picked, topic)
	}
	body := kafkaString(group)
	body = appendInt32(body, int32(len(picked)))
	for _, topic := range picked {
		body = append(body, kafkaString(topic.Name)...)
		body = appendInt32(body, int32(len(topic.partIDs)))
		for _, part := range topic.partIDs {
			body = appendInt32(body, part)
		}
	}
	return body
}

func listOffsetsBody(ver int16, topics map[string][]int32) []byte {
	body := appendInt32(nil, -1)
	if ver >= 2 {
		body = append(body, 0)
	}
	body = appendInt32(body, int32(len(topics)))
	for topic, parts := range topics {
		body = append(body, kafkaString(topic)...)
		body = appendInt32(body, int32(len(parts)))
		for _, part := range parts {
			body = appendInt32(body, part)
			body = appendInt64(body, -1)
		}
	}
	return body
}

func chunks(names []string, size int) [][]string {
	if size < 1 {
		size = 1
	}
	var out [][]string
	for len(names) > 0 {
		n := size
		if len(names) < n {
			n = len(names)
		}
		out = append(out, names[:n])
		names = names[n:]
	}
	return out
}

func summarizeLag(committed, ends map[string]map[int32]int64) (total, maxPart int64, topic string, part int32, topics int, ok bool) {
	maxPart = -1
	part = -1
	seen := map[string]struct{}{}
	for name, parts := range committed {
		for id, commit := range parts {
			end, endOK := ends[name][id]
			if !endOK || commit < 0 || end < 0 {
				continue
			}
			lag := end - commit
			if lag < 0 {
				lag = 0
			}
			ok = true
			total += lag
			seen[name] = struct{}{}
			if lag > maxPart {
				maxPart = lag
				topic = name
				part = id
			}
		}
	}
	if !ok {
		return -1, -1, "", -1, 0, false
	}
	return total, maxPart, topic, part, len(seen), true
}

type groupInfo struct {
	State   string
	Members int
}

// ParseListGroups 解析 ListGroups v0。
func ParseListGroups(body []byte) ([]string, error) {
	return parseListGroups(body, 0)
}

func parseListGroups(body []byte, ver int16) ([]string, error) {
	r := reader{b: body}
	if ver >= 1 {
		if _, err := r.i32(); err != nil {
			return nil, err
		}
	}
	code, err := r.i16()
	if err != nil || code != 0 {
		return nil, fmt.Errorf("Kafka 消费组列表失败")
	}
	n, err := r.i32()
	if err != nil || n < 0 {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	out := make([]string, 0, n)
	for i := int32(0); i < n; i++ {
		name, err := r.str()
		if err != nil {
			return nil, err
		}
		if _, err := r.str(); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, nil
}

// ParseCoordinator 解析 FindCoordinator v0。
func ParseCoordinator(body []byte) (string, int32, error) {
	return parseCoordinator(body, 0)
}

func parseCoordinator(body []byte, ver int16) (string, int32, error) {
	r := reader{b: body}
	if ver >= 1 {
		if _, err := r.i32(); err != nil {
			return "", 0, err
		}
	}
	code, err := r.i16()
	if err != nil || code != 0 {
		return "", 0, fmt.Errorf("Kafka 找不到消费组协调者")
	}
	if ver >= 1 {
		if _, _, err := r.nullable(); err != nil {
			return "", 0, err
		}
	}
	if _, err := r.i32(); err != nil {
		return "", 0, err
	}
	host, err := r.str()
	if err != nil {
		return "", 0, err
	}
	port, err := r.i32()
	if err != nil || host == "" || port <= 0 {
		return "", 0, fmt.Errorf("Kafka 找不到消费组协调者")
	}
	return host, port, nil
}

// ParseOffsetFetch 解析 OffsetFetch v1，返回 topic、分区到已提交位移。
func ParseOffsetFetch(body []byte) (map[string]map[int32]int64, error) {
	return parseOffsetFetch(body, 1)
}

func parseOffsetFetch(body []byte, ver int16) (map[string]map[int32]int64, error) {
	r := reader{b: body}
	if ver >= 3 {
		if _, err := r.i32(); err != nil {
			return nil, err
		}
	}
	topics, err := r.i32()
	if err != nil || topics < 0 {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	out := map[string]map[int32]int64{}
	for i := int32(0); i < topics; i++ {
		name, err := r.str()
		if err != nil {
			return nil, err
		}
		parts, err := r.i32()
		if err != nil || parts < 0 {
			return nil, fmt.Errorf("Kafka 应答无效")
		}
		out[name] = map[int32]int64{}
		for j := int32(0); j < parts; j++ {
			part, err := r.i32()
			if err != nil {
				return nil, err
			}
			offset, err := r.i64()
			if err != nil {
				return nil, err
			}
			if _, _, err := r.nullable(); err != nil {
				return nil, err
			}
			if _, err := r.i16(); err != nil {
				return nil, err
			}
			out[name][part] = offset
		}
	}
	return out, nil
}

// ParseListOffsets 解析 ListOffsets v1，返回 topic、分区到日志末端。
func ParseListOffsets(body []byte) (map[string]map[int32]int64, error) {
	return parseListOffsets(body, 1)
}

func parseListOffsets(body []byte, ver int16) (map[string]map[int32]int64, error) {
	r := reader{b: body}
	if ver >= 2 {
		if _, err := r.i32(); err != nil {
			return nil, err
		}
	}
	topics, err := r.i32()
	if err != nil || topics < 0 {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	out := map[string]map[int32]int64{}
	for i := int32(0); i < topics; i++ {
		name, err := r.str()
		if err != nil {
			return nil, err
		}
		parts, err := r.i32()
		if err != nil || parts < 0 {
			return nil, fmt.Errorf("Kafka 应答无效")
		}
		out[name] = map[int32]int64{}
		for j := int32(0); j < parts; j++ {
			part, err := r.i32()
			if err != nil {
				return nil, err
			}
			code, err := r.i16()
			if err != nil {
				return nil, err
			}
			if _, err := r.i64(); err != nil {
				return nil, err
			}
			offset, err := r.i64()
			if err != nil {
				return nil, err
			}
			if code == 0 {
				out[name][part] = offset
			}
		}
	}
	return out, nil
}

// ParseDescribeGroups 解析 DescribeGroups v0 或 v1。
func ParseDescribeGroups(body []byte, ver int16) (map[string]groupInfo, error) {
	r := reader{b: body}
	if ver >= 1 {
		if _, err := r.i32(); err != nil {
			return nil, err
		}
	}
	n, err := r.i32()
	if err != nil || n < 0 {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	out := map[string]groupInfo{}
	for i := int32(0); i < n; i++ {
		code, err := r.i16()
		if err != nil {
			return nil, err
		}
		id, err := r.str()
		if err != nil {
			return nil, err
		}
		state, err := r.str()
		if err != nil {
			return nil, err
		}
		if _, err := r.str(); err != nil {
			return nil, err
		}
		if _, err := r.str(); err != nil {
			return nil, err
		}
		members, err := r.i32()
		if err != nil || members < 0 {
			return nil, fmt.Errorf("Kafka 应答无效")
		}
		for j := int32(0); j < members; j++ {
			if _, err := r.str(); err != nil {
				return nil, err
			}
			if _, err := r.str(); err != nil {
				return nil, err
			}
			if _, err := r.str(); err != nil {
				return nil, err
			}
			if err := r.skipBytes(); err != nil {
				return nil, err
			}
			if err := r.skipBytes(); err != nil {
				return nil, err
			}
		}
		if ver >= 1 {
			if _, err := r.i32(); err != nil {
				return nil, err
			}
		}
		if code == 0 {
			out[id] = groupInfo{State: state, Members: int(members)}
		}
	}
	return out, nil
}
