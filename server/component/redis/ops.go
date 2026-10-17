package redis

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"fluxguard/server/component/conn"
	"fluxguard/server/component/netx"
	"fluxguard/server/component/profile"
)

const (
	redisOpTimeout = 20 * time.Second
	scanPage       = 100
	scanRounds     = 16
	valueCap       = 4096
	memberCap      = 50
	keyArgLimit    = 1024
	matchArgLimit  = 256
	cursorArgLimit = 4096
)

var redisCursorDigits = regexp.MustCompile(`^[0-9]+$`)

// Exec 按库浏览键、按类型读取，并删除或设置过期时间。
// 查找使用 SCAN，不使用 KEYS。集群会走每个主节点。
func (probe) Exec(ctx context.Context, target conn.Open, command string, args map[string]string) (string, error) {
	if args == nil {
		args = map[string]string{}
	}
	if err := checkRedisOp(command, args); err != nil {
		return "", err
	}
	db := strings.TrimSpace(args["database"])
	if db == "" {
		db = "0"
	}
	ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()
	link, err := openRedis(ctx, target, db)
	if err != nil {
		return "", err
	}
	var body any
	switch command {
	case "databases":
		body, err = link.databases(ctx)
	case "scan":
		body, err = link.scan(ctx, strings.TrimSpace(args["match"]), strings.TrimSpace(args["cursor"]))
	case "key":
		body, err = link.readKey(ctx, strings.TrimSpace(args["name"]))
	case "delete":
		body, err = link.removeKey(ctx, strings.TrimSpace(args["name"]))
	case "expire":
		body, err = link.touchExpire(ctx, strings.TrimSpace(args["name"]), strings.TrimSpace(args["seconds"]))
	default:
		err = fmt.Errorf("这种组件没有这项操作")
	}
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("Redis 应答无效")
	}
	return string(out), nil
}

type redisOpSpec struct {
	need  []string
	allow map[string]struct{}
}

func checkRedisOp(command string, args map[string]string) error {
	spec, ok := redisOps[command]
	if !ok {
		return fmt.Errorf("这种组件没有这项操作")
	}
	if len(args) > 6 {
		return fmt.Errorf("操作参数过多")
	}
	for key := range args {
		if _, allowed := spec.allow[key]; !allowed {
			return fmt.Errorf("这项操作没有这个参数")
		}
	}
	for _, key := range spec.need {
		if strings.TrimSpace(args[key]) == "" {
			switch key {
			case "name":
				return fmt.Errorf("请填写键名")
			case "seconds":
				return fmt.Errorf("请填写过期时间")
			default:
				return fmt.Errorf("请填写%s", key)
			}
		}
	}
	if db := strings.TrimSpace(args["database"]); db != "" {
		n, err := strconv.Atoi(db)
		if err != nil || n < 0 || n > 255 {
			return fmt.Errorf("库号应为 0 到 255")
		}
	}
	if name := strings.TrimSpace(args["name"]); name != "" && len(name) > keyArgLimit {
		return fmt.Errorf("键名过长")
	}
	if match := strings.TrimSpace(args["match"]); match != "" {
		if len(match) > matchArgLimit || strings.ContainsAny(match, "\r\n") {
			return fmt.Errorf("匹配条件无效")
		}
	}
	if cursor := strings.TrimSpace(args["cursor"]); len(cursor) > cursorArgLimit {
		return fmt.Errorf("游标无效")
	}
	if command == "expire" {
		seconds := strings.TrimSpace(args["seconds"])
		n, err := strconv.ParseInt(seconds, 10, 64)
		if err != nil || n < 0 || n > 2147483647 {
			return fmt.Errorf("过期时间应为 0 到 2147483647 秒，0 表示取消过期")
		}
	}
	return nil
}

var redisOps = map[string]redisOpSpec{
	"databases": {allow: map[string]struct{}{}},
	"scan":      {allow: map[string]struct{}{"database": {}, "match": {}, "cursor": {}}},
	"key":       {need: []string{"name"}, allow: map[string]struct{}{"database": {}, "name": {}}},
	"delete":    {need: []string{"name"}, allow: map[string]struct{}{"database": {}, "name": {}}},
	"expire":    {need: []string{"name", "seconds"}, allow: map[string]struct{}{"database": {}, "name": {}, "seconds": {}}},
}

type redisDatabase struct {
	Name    string `json:"name"`
	Keys    int64  `json:"keys"`
	Expires int64  `json:"expires"`
}

type redisDatabaseList struct {
	Databases []redisDatabase `json:"databases"`
	Note      string          `json:"note,omitempty"`
}

type redisScanResult struct {
	Database string   `json:"database"`
	Keys     []string `json:"keys"`
	Cursor   string   `json:"cursor"`
	Note     string   `json:"note,omitempty"`
}

type redisPair struct {
	Field string `json:"field"`
	Value string `json:"value,omitempty"`
	Score string `json:"score,omitempty"`
}

type redisKeyView struct {
	Name   string      `json:"name"`
	Type   string      `json:"type"`
	TTL    int64       `json:"ttl"`
	Count  int64       `json:"count,omitempty"`
	Text   string      `json:"text,omitempty"`
	Items  []string    `json:"items,omitempty"`
	Fields []redisPair `json:"fields,omitempty"`
	Note   string      `json:"note,omitempty"`
}

type redisDone struct {
	OK bool `json:"ok"`
}

type redisLink struct {
	auth    string
	user    string
	secret  string
	db      string
	cluster bool
	nodes   []string
	slots   []redisSlot
}

type redisSlot struct {
	start int
	end   int
	addr  string
}

func openRedis(ctx context.Context, target conn.Open, db string) (*redisLink, error) {
	deploy, auth := profile.Split(target.OptionText)
	if auth == "" {
		auth = inferredAuth(target)
	}
	if deploy == "cluster" && db != "0" {
		return nil, fmt.Errorf("集群只有 0 号库")
	}
	link := &redisLink{auth: auth, user: target.UserName, secret: target.Secret, db: db, cluster: deploy == "cluster"}
	switch deploy {
	case "cluster":
		return link.openCluster(ctx, target)
	case "sentinel":
		return link.openSentinel(ctx, target)
	default:
		addr, err := netx.HostPort(target.Address, 6379)
		if err != nil {
			return nil, err
		}
		link.nodes = []string{addr}
		return link, nil
	}
}

func (link *redisLink) openCluster(ctx context.Context, target conn.Open) (*redisLink, error) {
	addrs, err := netx.HostPorts(target.Address, 6379)
	if err != nil {
		return nil, err
	}
	var last error
	for _, addr := range addrs {
		raw, err := link.dial(ctx, addr, false)
		if err != nil {
			last = err
			continue
		}
		value, err := commandAny(raw, "CLUSTER", "SLOTS")
		_ = raw.Close()
		if err != nil {
			last = err
			continue
		}
		slots, err := parseSlots(value)
		if err != nil {
			last = err
			continue
		}
		link.slots = slots
		link.nodes = masterAddrs(slots)
		return link, nil
	}
	if last == nil {
		last = fmt.Errorf("Redis 集群没有可用节点")
	}
	return nil, last
}

func (link *redisLink) openSentinel(ctx context.Context, target conn.Open) (*redisLink, error) {
	master := strings.TrimSpace(target.DatabaseName)
	if master == "" {
		return nil, fmt.Errorf("请填写主节点名")
	}
	addrs, err := netx.HostPorts(target.Address, 26379)
	if err != nil {
		return nil, err
	}
	var last error
	for _, addr := range addrs {
		raw, err := link.dial(ctx, addr, false)
		if err != nil {
			last = err
			continue
		}
		fields, err := commandList(raw, "SENTINEL", "MASTER", master)
		_ = raw.Close()
		if err != nil {
			last = err
			continue
		}
		view := ParseSentinel(fields)
		if view.Down {
			last = fmt.Errorf("哨兵看到主节点已下线")
			continue
		}
		if view.IP == "" {
			last = fmt.Errorf("哨兵没有返回主节点地址")
			continue
		}
		link.nodes = []string{net.JoinHostPort(view.IP, empty(view.Port, "6379"))}
		return link, nil
	}
	if last == nil {
		last = fmt.Errorf("没有可用的哨兵")
	}
	return nil, last
}

func (link *redisLink) dial(ctx context.Context, addr string, ask bool) (net.Conn, error) {
	raw, err := dial(ctx, addr)
	if err != nil {
		return nil, err
	}
	if err := authenticate(raw, link.auth, link.user, link.secret); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if ask {
		if _, err := command(raw, "ASKING"); err != nil {
			_ = raw.Close()
			return nil, err
		}
	}
	return raw, nil
}

func (link *redisLink) dialDB(ctx context.Context, addr string, ask bool) (net.Conn, error) {
	raw, err := link.dial(ctx, addr, ask)
	if err != nil {
		return nil, err
	}
	if link.db != "0" {
		if _, err := command(raw, "SELECT", link.db); err != nil {
			_ = raw.Close()
			return nil, err
		}
	}
	return raw, nil
}

func (link *redisLink) databases(ctx context.Context) (redisDatabaseList, error) {
	if link.cluster {
		return link.clusterDatabases(ctx)
	}
	raw, err := link.dial(ctx, link.nodes[0], false)
	if err != nil {
		return redisDatabaseList{}, err
	}
	defer raw.Close()
	body, err := command(raw, "INFO", "keyspace")
	if err != nil {
		return redisDatabaseList{}, err
	}
	sample := ParseInfo(body)
	count := 16
	if fields, err := commandList(raw, "CONFIG", "GET", "databases"); err == nil && len(fields) >= 2 {
		if n := int(atoi(fields[1])); n >= 1 && n <= 256 {
			count = n
		}
	}
	filled := fillDatabases(sample.DBs, count)
	return redisDatabaseList{Databases: copyDatabases(filled)}, nil
}

func (link *redisLink) clusterDatabases(ctx context.Context) (redisDatabaseList, error) {
	var keys, expires int64
	var missed int
	for _, addr := range link.nodes {
		raw, err := link.dial(ctx, addr, false)
		if err != nil {
			missed++
			continue
		}
		body, err := command(raw, "INFO", "keyspace")
		_ = raw.Close()
		if err != nil {
			missed++
			continue
		}
		for _, item := range ParseInfo(body).DBs {
			if item.Name == "db0" {
				keys += item.Keys
				expires += item.Expires
			}
		}
	}
	if missed == len(link.nodes) {
		return redisDatabaseList{}, fmt.Errorf("Redis 集群没有可用节点")
	}
	out := redisDatabaseList{Databases: []redisDatabase{{Name: "db0", Keys: keys, Expires: expires}}}
	out.Note = "集群只有 0 号库。查找会走每个主节点。"
	if missed > 0 {
		out.Note += "有主节点没有读到键数量。"
	}
	return out, nil
}

func (link *redisLink) scan(ctx context.Context, match, cursor string) (redisScanResult, error) {
	if match == "" {
		match = "*"
	}
	state, err := parseScanCursor(cursor, link.nodes)
	if err != nil {
		return redisScanResult{}, err
	}
	var keys []string
	for _, addr := range link.nodes {
		if len(keys) >= scanPage {
			break
		}
		cur := state[addr]
		if cur == "done" {
			continue
		}
		batch, next, err := link.scanAddr(ctx, addr, cur, match, scanPage-len(keys))
		if err != nil {
			return redisScanResult{}, err
		}
		keys = append(keys, batch...)
		state[addr] = next
	}
	out := redisScanResult{Database: link.db, Keys: keys, Cursor: encodeCursor(link.nodes, state)}
	if out.Keys == nil {
		out.Keys = []string{}
	}
	if out.Cursor != "0" {
		out.Note = "还有更多键。可以继续，或缩小匹配条件。"
	}
	return out, nil
}

func (link *redisLink) scanAddr(ctx context.Context, addr, cursor, match string, want int) ([]string, string, error) {
	raw, err := link.dialDB(ctx, addr, false)
	if err != nil {
		return nil, "", err
	}
	defer raw.Close()
	if cursor == "" {
		cursor = "0"
	}
	var keys []string
	for i := 0; i < scanRounds && len(keys) < want; i++ {
		value, err := commandAny(raw, "SCAN", cursor, "MATCH", match, "COUNT", "100")
		if err != nil {
			return nil, "", err
		}
		next, batch, err := parseScan(value)
		if err != nil {
			return nil, "", err
		}
		keys = append(keys, batch...)
		if next == "0" {
			return keys, "done", nil
		}
		cursor = next
	}
	if cursor == "0" {
		return keys, "done", nil
	}
	return keys, cursor, nil
}

func (link *redisLink) readKey(ctx context.Context, name string) (redisKeyView, error) {
	var view redisKeyView
	err := link.onKey(ctx, name, func(raw net.Conn) error {
		got, err := readKeyOn(raw, name)
		if err != nil {
			return err
		}
		view = got
		return nil
	})
	return view, err
}

func (link *redisLink) removeKey(ctx context.Context, name string) (redisDone, error) {
	err := link.onKey(ctx, name, func(raw net.Conn) error {
		if err := requireKey(raw, name); err != nil {
			return err
		}
		if _, err := command(raw, "DEL", name); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return redisDone{}, err
	}
	return redisDone{OK: true}, nil
}

func (link *redisLink) touchExpire(ctx context.Context, name, seconds string) (redisDone, error) {
	err := link.onKey(ctx, name, func(raw net.Conn) error {
		if err := requireKey(raw, name); err != nil {
			return err
		}
		var err error
		if seconds == "0" {
			_, err = command(raw, "PERSIST", name)
		} else {
			_, err = command(raw, "EXPIRE", name, seconds)
		}
		return err
	})
	if err != nil {
		return redisDone{}, err
	}
	return redisDone{OK: true}, nil
}

func (link *redisLink) onKey(ctx context.Context, name string, fn func(net.Conn) error) error {
	addr := link.addrFor(name)
	ask := false
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := link.dialDB(ctx, addr, ask)
		if err != nil {
			return err
		}
		err = fn(raw)
		_ = raw.Close()
		if err == nil {
			return nil
		}
		next, asking, ok := redirectOf(err)
		if !ok || attempt == 1 {
			return err
		}
		addr = next
		ask = asking
		last = err
	}
	if last == nil {
		last = fmt.Errorf("Redis 应答无效")
	}
	return last
}

func (link *redisLink) addrFor(name string) string {
	if !link.cluster || len(link.slots) == 0 {
		return link.nodes[0]
	}
	slot := hashSlot(name)
	for _, item := range link.slots {
		if slot >= item.start && slot <= item.end {
			return item.addr
		}
	}
	return link.nodes[0]
}

func readKeyOn(raw net.Conn, name string) (redisKeyView, error) {
	kind, err := command(raw, "TYPE", name)
	if err != nil {
		return redisKeyView{}, err
	}
	view := redisKeyView{Name: name, Type: kind, TTL: -2}
	if kind == "none" {
		view.Note = "键不存在。"
		return view, nil
	}
	ttl, err := command(raw, "TTL", name)
	if err != nil {
		return redisKeyView{}, err
	}
	view.TTL = atoi(ttl)
	switch kind {
	case "string":
		return readString(raw, view)
	case "hash":
		return readHash(raw, view)
	case "list":
		return readList(raw, view)
	case "set":
		return readSet(raw, view)
	case "zset":
		return readZSet(raw, view)
	default:
		view.Note = "这种类型只显示类型和过期时间。"
		return view, nil
	}
}

func readString(raw net.Conn, view redisKeyView) (redisKeyView, error) {
	n, err := command(raw, "STRLEN", view.Name)
	if err != nil {
		return redisKeyView{}, err
	}
	size := atoi(n)
	view.Count = size
	var text string
	if size > valueCap {
		text, err = command(raw, "GETRANGE", view.Name, "0", strconv.Itoa(valueCap-1))
	} else {
		text, err = command(raw, "GET", view.Name)
	}
	if err != nil {
		return redisKeyView{}, err
	}
	shown, notes := clipValue(text, size > valueCap, size)
	view.Text = shown
	view.Note = joinNotes(notes)
	return view, nil
}

func readHash(raw net.Conn, view redisKeyView) (redisKeyView, error) {
	n, err := command(raw, "HLEN", view.Name)
	if err != nil {
		return redisKeyView{}, err
	}
	view.Count = atoi(n)
	value, err := commandAny(raw, "HSCAN", view.Name, "0", "COUNT", strconv.Itoa(memberCap))
	if err != nil {
		return redisKeyView{}, err
	}
	_, flat, err := parseScan(value)
	if err != nil {
		return redisKeyView{}, err
	}
	var binary bool
	for i := 0; i+1 < len(flat) && len(view.Fields) < memberCap; i += 2 {
		field, fieldNotes := clipValue(flat[i], false, 0)
		item, itemNotes := clipValue(flat[i+1], false, 0)
		binary = binary || len(fieldNotes) > 0 || len(itemNotes) > 0
		view.Fields = append(view.Fields, redisPair{Field: field, Value: item})
	}
	if view.Count > int64(len(view.Fields)) {
		view.Note = fmt.Sprintf("只显示前 %d 项，共 %d 项。", len(view.Fields), view.Count)
	}
	if binary {
		view.Note = joinNotes([]string{view.Note, "部分内容不是文本，或超过显示长度。"})
	}
	return view, nil
}

func readList(raw net.Conn, view redisKeyView) (redisKeyView, error) {
	n, err := command(raw, "LLEN", view.Name)
	if err != nil {
		return redisKeyView{}, err
	}
	view.Count = atoi(n)
	value, err := commandAny(raw, "LRANGE", view.Name, "0", strconv.Itoa(memberCap-1))
	if err != nil {
		return redisKeyView{}, err
	}
	items, err := asStrings(value)
	if err != nil {
		return redisKeyView{}, err
	}
	view.Items, view.Note = clipItems(items, view.Count)
	return view, nil
}

func readSet(raw net.Conn, view redisKeyView) (redisKeyView, error) {
	n, err := command(raw, "SCARD", view.Name)
	if err != nil {
		return redisKeyView{}, err
	}
	view.Count = atoi(n)
	value, err := commandAny(raw, "SSCAN", view.Name, "0", "COUNT", strconv.Itoa(memberCap))
	if err != nil {
		return redisKeyView{}, err
	}
	_, items, err := parseScan(value)
	if err != nil {
		return redisKeyView{}, err
	}
	view.Items, view.Note = clipItems(items, view.Count)
	return view, nil
}

func readZSet(raw net.Conn, view redisKeyView) (redisKeyView, error) {
	n, err := command(raw, "ZCARD", view.Name)
	if err != nil {
		return redisKeyView{}, err
	}
	view.Count = atoi(n)
	value, err := commandAny(raw, "ZRANGE", view.Name, "0", strconv.Itoa(memberCap-1), "WITHSCORES")
	if err != nil {
		return redisKeyView{}, err
	}
	flat, err := asStrings(value)
	if err != nil {
		return redisKeyView{}, err
	}
	var binary bool
	for i := 0; i+1 < len(flat) && len(view.Fields) < memberCap; i += 2 {
		member, notes := clipValue(flat[i], false, 0)
		binary = binary || len(notes) > 0
		view.Fields = append(view.Fields, redisPair{Field: member, Score: flat[i+1]})
	}
	if view.Count > int64(len(view.Fields)) {
		view.Note = fmt.Sprintf("只显示前 %d 项，共 %d 项。", len(view.Fields), view.Count)
	}
	if binary {
		view.Note = joinNotes([]string{view.Note, "部分内容不是文本，或超过显示长度。"})
	}
	return view, nil
}

func requireKey(raw net.Conn, name string) error {
	n, err := command(raw, "EXISTS", name)
	if err != nil {
		return err
	}
	if n == "0" {
		return fmt.Errorf("键不存在")
	}
	return nil
}

func clipItems(items []string, total int64) ([]string, string) {
	var notes []string
	out := make([]string, 0, len(items))
	binary := false
	for _, item := range items {
		if len(out) >= memberCap {
			break
		}
		shown, itemNotes := clipValue(item, false, 0)
		binary = binary || len(itemNotes) > 0
		out = append(out, shown)
	}
	if total > int64(len(out)) {
		notes = append(notes, fmt.Sprintf("只显示前 %d 项，共 %d 项。", len(out), total))
	}
	if binary {
		notes = append(notes, "部分内容不是文本，或超过显示长度。")
	}
	return out, joinNotes(notes)
}

func clipValue(text string, truncated bool, total int64) (string, []string) {
	var notes []string
	if !utf8.ValidString(text) {
		raw := text
		if len(raw) > valueCap/2 {
			raw = raw[:valueCap/2]
			truncated = true
		}
		if truncated && total > 0 {
			notes = append(notes, fmt.Sprintf("只显示前一部分，共 %d 字节。", total))
		}
		notes = append(notes, "内容不是文本，按十六进制显示。")
		return hex.EncodeToString([]byte(raw)), notes
	}
	shown := text
	if len(shown) > valueCap {
		shown = shown[:valueCap]
		for shown != "" && !utf8.ValidString(shown) {
			shown = shown[:len(shown)-1]
		}
		truncated = true
	}
	if truncated {
		if total > 0 {
			notes = append(notes, fmt.Sprintf("只显示前 %d 字节，共 %d 字节。", len(shown), total))
		} else {
			notes = append(notes, "只显示前一部分。")
		}
	}
	return shown, notes
}

func joinNotes(notes []string) string {
	var kept []string
	for _, note := range notes {
		if note != "" {
			kept = append(kept, note)
		}
	}
	return strings.Join(kept, "")
}

func parseScan(value any) (string, []string, error) {
	row, ok := value.([]any)
	if !ok || len(row) != 2 {
		return "", nil, fmt.Errorf("Redis 应答无效")
	}
	cursor, ok := row[0].(string)
	if !ok || !redisCursorDigits.MatchString(cursor) {
		return "", nil, fmt.Errorf("Redis 应答无效")
	}
	items, err := asStrings(row[1])
	if err != nil {
		return "", nil, err
	}
	return cursor, items, nil
}

func asStrings(value any) ([]string, error) {
	row, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("Redis 应答无效")
	}
	out := make([]string, len(row))
	for i, item := range row {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("Redis 应答无效")
		}
		out[i] = text
	}
	return out, nil
}

func parseSlots(value any) ([]redisSlot, error) {
	rows, ok := value.([]any)
	if !ok || len(rows) == 0 {
		return nil, fmt.Errorf("Redis 集群没有槽位")
	}
	var out []redisSlot
	for _, row := range rows {
		fields, ok := row.([]any)
		if !ok || len(fields) < 3 {
			continue
		}
		start, err := asInt(fields[0])
		if err != nil {
			continue
		}
		end, err := asInt(fields[1])
		if err != nil || end < start {
			continue
		}
		master, ok := fields[2].([]any)
		if !ok || len(master) < 2 {
			continue
		}
		ip, _ := master[0].(string)
		port, _ := master[1].(string)
		if ip == "" || port == "" {
			continue
		}
		out = append(out, redisSlot{start: start, end: end, addr: net.JoinHostPort(ip, port)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("Redis 集群没有主节点")
	}
	return out, nil
}

func asInt(value any) (int, error) {
	text, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("Redis 应答无效")
	}
	return strconv.Atoi(text)
}

func masterAddrs(slots []redisSlot) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range slots {
		if _, ok := seen[item.addr]; ok {
			continue
		}
		seen[item.addr] = struct{}{}
		out = append(out, item.addr)
	}
	return out
}

func parseScanCursor(cursor string, masters []string) (map[string]string, error) {
	state := map[string]string{}
	if cursor == "" || cursor == "0" {
		for _, addr := range masters {
			state[addr] = "0"
		}
		return state, nil
	}
	if redisCursorDigits.MatchString(cursor) {
		if len(masters) != 1 {
			return nil, fmt.Errorf("游标无效")
		}
		state[masters[0]] = cursor
		return state, nil
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(cursor), &raw); err != nil || len(raw) == 0 {
		return nil, fmt.Errorf("游标无效")
	}
	allowed := map[string]struct{}{}
	for _, addr := range masters {
		allowed[addr] = struct{}{}
		state[addr] = "done"
	}
	for addr, cur := range raw {
		if _, ok := allowed[addr]; !ok || !redisCursorDigits.MatchString(cur) {
			return nil, fmt.Errorf("游标无效")
		}
		state[addr] = cur
	}
	return state, nil
}

func encodeCursor(masters []string, state map[string]string) string {
	pending := map[string]string{}
	for _, addr := range masters {
		cur := state[addr]
		if cur == "" || cur == "done" {
			continue
		}
		pending[addr] = cur
	}
	if len(pending) == 0 {
		return "0"
	}
	if len(masters) == 1 {
		for _, cur := range pending {
			if cur == "0" {
				return "0"
			}
			return cur
		}
	}
	raw, err := json.Marshal(pending)
	if err != nil {
		return "0"
	}
	return string(raw)
}

func copyDatabases(items []DbStat) []redisDatabase {
	out := make([]redisDatabase, len(items))
	for i, item := range items {
		out[i] = redisDatabase{Name: item.Name, Keys: item.Keys, Expires: item.Expires}
	}
	return out
}

func redirectOf(err error) (string, bool, bool) {
	if err == nil {
		return "", false, false
	}
	text := err.Error()
	ask := strings.HasPrefix(text, "ASK ")
	if !ask && !strings.HasPrefix(text, "MOVED ") {
		return "", false, false
	}
	fields := strings.Fields(text)
	if len(fields) < 3 || !strings.Contains(fields[len(fields)-1], ":") {
		return "", false, false
	}
	return fields[len(fields)-1], ask, true
}

// hashSlot 计算集群槽位。花括号里的内容是哈希标签。
func hashSlot(key string) int {
	if start := strings.IndexByte(key, '{'); start >= 0 {
		if end := strings.IndexByte(key[start+1:], '}'); end > 0 {
			key = key[start+1 : start+1+end]
		}
	}
	return int(crc16([]byte(key)) % 16384)
}

func crc16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
