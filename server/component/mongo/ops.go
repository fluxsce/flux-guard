package mongo

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"fluxguard/server/component/conn"
	"fluxguard/server/component/netx"
	"fluxguard/server/component/row"
)

const (
	opTimeout      = 20 * time.Second
	opArgLimit     = 512
	dbListLimit    = 500
	collListLimit  = 500
	indexListLimit = 64
)

// Exec 执行已登记的库、集合、索引维护，以及终止正在执行的操作。
func (probe) Exec(ctx context.Context, target conn.Open, command string, args map[string]string) (string, error) {
	if args == nil {
		args = map[string]string{}
	}
	if err := checkMongoOp(command, args); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	plan, err := authMode(target)
	if err != nil {
		return "", err
	}
	addr, err := netx.HostPort(target.Address, 27017)
	if err != nil {
		return "", err
	}
	raw, err := dialMongo(ctx, addr, plan)
	if err != nil {
		return "", err
	}
	defer raw.Close()
	if plan.mechanism != "" {
		if err := authenticate(raw, plan); err != nil {
			return "", err
		}
	}
	body, err := runMongoOp(raw, command, args)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("MongoDB 应答无效")
	}
	return string(out), nil
}

type mongoOp struct {
	need  []string
	allow map[string]struct{}
}

func checkMongoOp(command string, args map[string]string) error {
	spec, ok := mongoOps[command]
	if !ok {
		return fmt.Errorf("这种组件没有这项操作")
	}
	if len(args) > 8 {
		return fmt.Errorf("操作参数过多")
	}
	for key, value := range args {
		if _, allowed := spec.allow[key]; !allowed {
			return fmt.Errorf("这项操作没有这个参数")
		}
		limit := opArgLimit
		if key == "filter" {
			limit = 8192
		}
		if len(value) > limit {
			return fmt.Errorf("操作参数过长")
		}
	}
	labels := map[string]string{
		"database": "库名", "collection": "集合名", "name": "名称", "keys": "索引键", "op": "操作编号",
	}
	for _, key := range spec.need {
		if strings.TrimSpace(args[key]) == "" {
			return fmt.Errorf("请填写%s", labels[key])
		}
	}
	if db := strings.TrimSpace(args["database"]); db != "" {
		if err := validDatabase(db); err != nil {
			return err
		}
	}
	writeColl := command != "stats" && command != "indexes" && command != "find" && command != "count" && command != "explain"
	if coll := strings.TrimSpace(args["collection"]); coll != "" {
		if err := validCollection(coll, writeColl); err != nil {
			return err
		}
	}
	switch command {
	case "dropDatabase":
		if protectedDatabase(args["database"]) {
			return fmt.Errorf("不能删除 admin、local 或 config")
		}
	case "renameCollection":
		if err := validCollection(strings.TrimSpace(args["name"]), true); err != nil {
			return err
		}
		if strings.TrimSpace(args["name"]) == strings.TrimSpace(args["collection"]) {
			return fmt.Errorf("新名称与原来相同")
		}
	case "createIndex":
		keys, err := parseIndexKeys(args["keys"])
		if err != nil {
			return err
		}
		if name := strings.TrimSpace(args["name"]); name != "" {
			if err := validIndexName(name); err != nil {
				return err
			}
			if name == "_id_" {
				return fmt.Errorf("不能使用 _id_ 作为索引名")
			}
		}
		if flagArg(args["unique"]) != nil || flagArg(args["sparse"]) != nil {
			return fmt.Errorf("开关只能是 Y 或 N")
		}
		ttl, err := ttlArg(args["ttl"])
		if err != nil {
			return err
		}
		if ttl >= 0 && len(keys) != 1 {
			return fmt.Errorf("过期索引只能有一个字段")
		}
	case "dropIndex":
		if err := validIndexName(strings.TrimSpace(args["name"])); err != nil {
			return err
		}
		if strings.TrimSpace(args["name"]) == "_id_" {
			return fmt.Errorf("不能删除 _id_ 索引")
		}
	case "killOp":
		if _, err := opIDArg(args["op"]); err != nil {
			return err
		}
	case "find":
		if err := queryFilter(args["filter"]); err != nil {
			return err
		}
		if _, err := queryLimit(args["limit"]); err != nil {
			return err
		}
		if strings.HasPrefix(strings.TrimSpace(args["collection"]), "system.") {
			return fmt.Errorf("不能查询系统集合")
		}
	case "count", "explain":
		if err := queryFilter(args["filter"]); err != nil {
			return err
		}
		if strings.HasPrefix(strings.TrimSpace(args["collection"]), "system.") {
			return fmt.Errorf("不能查询系统集合")
		}
	}
	return nil
}

func flagArg(value string) error {
	switch strings.TrimSpace(value) {
	case "", "Y", "N":
		return nil
	default:
		return fmt.Errorf("开关只能是 Y 或 N")
	}
}

func ttlArg(value string) (int32, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return -1, nil
	}
	n, err := strconv.ParseInt(value, 10, 32)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("过期秒数无效")
	}
	return int32(n), nil
}

func opIDArg(value string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("操作编号无效")
	}
	return n, nil
}

var mongoOps = map[string]mongoOp{
	"databases":        {allow: map[string]struct{}{}},
	"collections":      {need: []string{"database"}, allow: setOf("database")},
	"stats":            {need: []string{"database", "collection"}, allow: setOf("database", "collection")},
	"indexes":          {need: []string{"database", "collection"}, allow: setOf("database", "collection")},
	"createDatabase":   {need: []string{"database", "collection"}, allow: setOf("database", "collection")},
	"dropDatabase":     {need: []string{"database"}, allow: setOf("database")},
	"createCollection": {need: []string{"database", "collection"}, allow: setOf("database", "collection")},
	"dropCollection":   {need: []string{"database", "collection"}, allow: setOf("database", "collection")},
	"renameCollection": {need: []string{"database", "collection", "name"}, allow: setOf("database", "collection", "name")},
	"createIndex":      {need: []string{"database", "collection", "keys"}, allow: setOf("database", "collection", "keys", "name", "unique", "sparse", "ttl")},
	"dropIndex":        {need: []string{"database", "collection", "name"}, allow: setOf("database", "collection", "name")},
	"killOp":           {need: []string{"op"}, allow: setOf("op")},
	"find":             {need: []string{"database", "collection"}, allow: setOf("database", "collection", "filter", "limit")},
	"count":            {need: []string{"database", "collection"}, allow: setOf("database", "collection", "filter")},
	"explain":          {need: []string{"database", "collection"}, allow: setOf("database", "collection", "filter")},
	"profile":          {need: []string{"database"}, allow: setOf("database")},
}

func setOf(keys ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		out[key] = struct{}{}
	}
	return out
}

func runMongoOp(raw net.Conn, command string, args map[string]string) (any, error) {
	switch command {
	case "databases":
		return opDatabases(raw)
	case "collections":
		return opCollections(raw, strings.TrimSpace(args["database"]))
	case "stats":
		return opStats(raw, strings.TrimSpace(args["database"]), strings.TrimSpace(args["collection"]))
	case "indexes":
		return opIndexes(raw, strings.TrimSpace(args["database"]), strings.TrimSpace(args["collection"]))
	case "createDatabase", "createCollection":
		return opCreate(raw, strings.TrimSpace(args["database"]), strings.TrimSpace(args["collection"]))
	case "dropDatabase":
		return opDropDatabase(raw, strings.TrimSpace(args["database"]))
	case "dropCollection":
		return opDropCollection(raw, strings.TrimSpace(args["database"]), strings.TrimSpace(args["collection"]))
	case "renameCollection":
		return opRename(raw, strings.TrimSpace(args["database"]), strings.TrimSpace(args["collection"]), strings.TrimSpace(args["name"]))
	case "createIndex":
		return opCreateIndex(raw, args)
	case "dropIndex":
		return opDropIndex(raw, strings.TrimSpace(args["database"]), strings.TrimSpace(args["collection"]), strings.TrimSpace(args["name"]))
	case "killOp":
		id, err := opIDArg(args["op"])
		if err != nil {
			return nil, err
		}
		return opKill(raw, id)
	case "find":
		limit, err := queryLimit(args["limit"])
		if err != nil {
			return nil, err
		}
		return opFind(raw, strings.TrimSpace(args["database"]), strings.TrimSpace(args["collection"]), args["filter"], limit)
	case "count":
		return opCount(raw, strings.TrimSpace(args["database"]), strings.TrimSpace(args["collection"]), args["filter"])
	case "explain":
		return opExplain(raw, strings.TrimSpace(args["database"]), strings.TrimSpace(args["collection"]), args["filter"])
	case "profile":
		return opProfile(raw, strings.TrimSpace(args["database"]))
	default:
		return nil, fmt.Errorf("这种组件没有这项操作")
	}
}

type mongoDBRow struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
	Empty bool   `json:"empty,omitempty"`
}

type mongoDatabases struct {
	Databases []mongoDBRow `json:"databases"`
	Note      string       `json:"note,omitempty"`
}

func opDatabases(raw net.Conn) (mongoDatabases, error) {
	doc, err := command(raw, bsonDoc(bsonInt32("listDatabases", 1), bsonString("$db", "admin")))
	if err != nil {
		return mongoDatabases{}, err
	}
	var out []mongoDBRow
	for _, item := range asSlice(dig(doc, "databases")) {
		rawItem, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := text(dig(rawItem, "name"))
		if err := validDatabase(name); err != nil {
			continue
		}
		out = append(out, mongoDBRow{Name: name, Bytes: int64(num(dig(rawItem, "sizeOnDisk"))), Empty: truthy(dig(rawItem, "empty"))})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes == out[j].Bytes {
			return out[i].Name < out[j].Name
		}
		return out[i].Bytes > out[j].Bytes
	})
	note := ""
	if len(out) > dbListLimit {
		note = fmt.Sprintf("库共 %d 个，这里列出占用最高的前 %d 个。", len(out), dbListLimit)
		out = out[:dbListLimit]
	}
	if out == nil {
		out = []mongoDBRow{}
	}
	return mongoDatabases{Databases: out, Note: note}, nil
}

type mongoCollRow struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type mongoCollections struct {
	Database    string         `json:"database"`
	Collections []mongoCollRow `json:"collections"`
	Note        string         `json:"note,omitempty"`
}

func opCollections(raw net.Conn, db string) (mongoCollections, error) {
	doc, err := command(raw, bsonDoc(bsonInt32("listCollections", 1), bsonBool("nameOnly", false), bsonString("$db", db)))
	if err != nil {
		return mongoCollections{}, err
	}
	batch, truncated, err := readBatches(raw, doc, db, "$cmd.listCollections", collListLimit)
	if err != nil {
		return mongoCollections{}, err
	}
	var out []mongoCollRow
	for _, item := range batch {
		name := text(dig(item, "name"))
		if name == "" || strings.ContainsRune(name, 0) {
			continue
		}
		kind := text(dig(item, "type"))
		if kind == "" {
			kind = "collection"
		}
		out = append(out, mongoCollRow{Name: row.Cut(name, 200), Type: kind})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	note := ""
	if truncated {
		note = fmt.Sprintf("集合很多，这里只列出前 %d 个。", collListLimit)
	}
	if out == nil {
		out = []mongoCollRow{}
	}
	return mongoCollections{Database: db, Collections: out, Note: note}, nil
}

type mongoStats struct {
	Database       string `json:"database"`
	Collection     string `json:"collection"`
	NS             string `json:"ns"`
	Count          int64  `json:"count"`
	Size           int64  `json:"size"`
	AvgObjSize     int64  `json:"avgObjSize"`
	StorageSize    int64  `json:"storageSize"`
	NIndexes       int64  `json:"nindexes"`
	TotalIndexSize int64  `json:"totalIndexSize"`
	Capped         bool   `json:"capped,omitempty"`
}

func opStats(raw net.Conn, db, coll string) (mongoStats, error) {
	doc, err := command(raw, bsonDoc(bsonString("collStats", coll), bsonInt32("scale", 1), bsonString("$db", db)))
	if err != nil {
		return mongoStats{}, err
	}
	return mongoStats{
		Database: db, Collection: coll, NS: db + "." + coll,
		Count: int64(num(dig(doc, "count"))), Size: int64(num(dig(doc, "size"))),
		AvgObjSize: int64(num(dig(doc, "avgObjSize"))), StorageSize: int64(num(dig(doc, "storageSize"))),
		NIndexes: int64(num(dig(doc, "nindexes"))), TotalIndexSize: int64(num(dig(doc, "totalIndexSize"))),
		Capped: truthy(dig(doc, "capped")),
	}, nil
}

type mongoIndexRow struct {
	Name    string `json:"name"`
	Key     string `json:"key"`
	Unique  bool   `json:"unique,omitempty"`
	Sparse  bool   `json:"sparse,omitempty"`
	Hidden  bool   `json:"hidden,omitempty"`
	TTL     int64  `json:"ttl,omitempty"`
	Partial bool   `json:"partial,omitempty"`
}

type mongoIndexes struct {
	Database   string          `json:"database"`
	Collection string          `json:"collection"`
	Indexes    []mongoIndexRow `json:"indexes"`
	Note       string          `json:"note,omitempty"`
}

func opIndexes(raw net.Conn, db, coll string) (mongoIndexes, error) {
	doc, err := command(raw, bsonDoc(bsonString("listIndexes", coll), bsonString("$db", db)))
	if err != nil {
		return mongoIndexes{}, err
	}
	batch, truncated, err := readBatches(raw, doc, db, coll, indexListLimit)
	if err != nil {
		return mongoIndexes{}, err
	}
	var out []mongoIndexRow
	for _, item := range batch {
		name := text(dig(item, "name"))
		if name == "" {
			continue
		}
		ttl, hasTTL := intOf(dig(item, "expireAfterSeconds"))
		rowItem := mongoIndexRow{
			Name: row.Cut(name, 128), Key: keyText(dig(item, "key")),
			Unique: truthy(dig(item, "unique")), Sparse: truthy(dig(item, "sparse")),
			Hidden: truthy(dig(item, "hidden")), Partial: dig(item, "partialFilterExpression") != nil,
		}
		if hasTTL {
			rowItem.TTL = ttl
		}
		out = append(out, rowItem)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == "_id_" {
			return true
		}
		if out[j].Name == "_id_" {
			return false
		}
		return out[i].Name < out[j].Name
	})
	note := ""
	if truncated {
		note = fmt.Sprintf("索引很多，这里只列出前 %d 个。", indexListLimit)
	}
	if out == nil {
		out = []mongoIndexRow{}
	}
	return mongoIndexes{Database: db, Collection: coll, Indexes: out, Note: note}, nil
}

type mongoAck struct {
	OK bool `json:"ok"`
}

func opCreate(raw net.Conn, db, coll string) (mongoAck, error) {
	_, err := command(raw, bsonDoc(bsonString("create", coll), bsonString("$db", db)))
	if err != nil {
		return mongoAck{}, err
	}
	return mongoAck{OK: true}, nil
}

func opDropDatabase(raw net.Conn, db string) (mongoAck, error) {
	_, err := command(raw, bsonDoc(bsonInt32("dropDatabase", 1), bsonString("$db", db)))
	if err != nil {
		return mongoAck{}, err
	}
	return mongoAck{OK: true}, nil
}

func opDropCollection(raw net.Conn, db, coll string) (mongoAck, error) {
	_, err := command(raw, bsonDoc(bsonString("drop", coll), bsonString("$db", db)))
	if err != nil {
		return mongoAck{}, err
	}
	return mongoAck{OK: true}, nil
}

func opRename(raw net.Conn, db, coll, name string) (mongoAck, error) {
	_, err := command(raw, bsonDoc(
		bsonString("renameCollection", db+"."+coll),
		bsonString("to", db+"."+name),
		bsonBool("dropTarget", false),
		bsonString("$db", "admin"),
	))
	if err != nil {
		return mongoAck{}, err
	}
	return mongoAck{OK: true}, nil
}

func opCreateIndex(raw net.Conn, args map[string]string) (mongoAck, error) {
	db := strings.TrimSpace(args["database"])
	coll := strings.TrimSpace(args["collection"])
	keys, err := parseIndexKeys(args["keys"])
	if err != nil {
		return mongoAck{}, err
	}
	ttl, err := ttlArg(args["ttl"])
	if err != nil {
		return mongoAck{}, err
	}
	if ttl >= 0 && len(keys) != 1 {
		return mongoAck{}, fmt.Errorf("过期索引只能有一个字段")
	}
	name := strings.TrimSpace(args["name"])
	if name == "" {
		name = indexName(keys)
	}
	if err := validIndexName(name); err != nil {
		return mongoAck{}, err
	}
	elems := [][]byte{bsonInt32(keys[0].field, keys[0].dir)}
	for _, key := range keys[1:] {
		elems = append(elems, bsonInt32(key.field, key.dir))
	}
	spec := [][]byte{
		bsonSub("key", bsonDoc(elems...)),
		bsonString("name", name),
	}
	if strings.TrimSpace(args["unique"]) == "Y" {
		spec = append(spec, bsonBool("unique", true))
	}
	if strings.TrimSpace(args["sparse"]) == "Y" {
		spec = append(spec, bsonBool("sparse", true))
	}
	if ttl >= 0 {
		spec = append(spec, bsonInt32("expireAfterSeconds", ttl))
	}
	_, err = command(raw, bsonDoc(
		bsonString("createIndexes", coll),
		bsonArray("indexes", bsonDoc(spec...)),
		bsonString("$db", db),
	))
	if err != nil {
		return mongoAck{}, err
	}
	return mongoAck{OK: true}, nil
}

func opDropIndex(raw net.Conn, db, coll, name string) (mongoAck, error) {
	_, err := command(raw, bsonDoc(
		bsonString("dropIndexes", coll),
		bsonString("index", name),
		bsonString("$db", db),
	))
	if err != nil {
		return mongoAck{}, err
	}
	return mongoAck{OK: true}, nil
}

func opKill(raw net.Conn, id int64) (mongoAck, error) {
	_, err := command(raw, bsonDoc(bsonInt32("killOp", 1), bsonNumber("op", id), bsonString("$db", "admin")))
	if err != nil {
		return mongoAck{}, err
	}
	return mongoAck{OK: true}, nil
}

func command(raw net.Conn, doc []byte) (map[string]any, error) {
	resp, err := call(raw, doc)
	if err != nil {
		return nil, err
	}
	if err := opFail(resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func opFail(doc map[string]any) error {
	if num(dig(doc, "ok")) >= 1 {
		return nil
	}
	msg := text(dig(doc, "errmsg"))
	if msg == "" {
		return fmt.Errorf("MongoDB 操作失败")
	}
	return fmt.Errorf("%s", row.Cut(msg, 200))
}

func readBatches(raw net.Conn, doc map[string]any, db, fallback string, limit int) ([]map[string]any, bool, error) {
	if dig(doc, "cursor") == nil {
		return nil, false, fmt.Errorf("MongoDB 应答无效")
	}
	var out []map[string]any
	truncated := false
	for round := 0; round < 30; round++ {
		field := "firstBatch"
		if round > 0 {
			field = "nextBatch"
		}
		for _, item := range asSlice(dig(doc, "cursor", field)) {
			rawItem, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if len(out) >= limit {
				truncated = true
				break
			}
			out = append(out, rawItem)
		}
		id, _ := intOf(dig(doc, "cursor", "id"))
		if id == 0 || truncated {
			return out, truncated, nil
		}
		coll := cursorCollection(text(dig(doc, "cursor", "ns")), fallback)
		var err error
		doc, err = command(raw, bsonDoc(
			bsonInt64("getMore", id),
			bsonString("collection", coll),
			bsonInt32("batchSize", 100),
			bsonString("$db", db),
		))
		if err != nil {
			return nil, false, err
		}
	}
	return out, true, nil
}

func cursorCollection(ns, fallback string) string {
	_, coll, ok := strings.Cut(ns, ".")
	if !ok || coll == "" {
		return fallback
	}
	return coll
}

type indexField struct {
	field string
	dir   int32
}

func parseIndexKeys(raw string) ([]indexField, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("请填写索引键")
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 32 {
		return nil, fmt.Errorf("索引键过多")
	}
	var out []indexField
	seen := map[string]struct{}{}
	for _, part := range parts {
		field, dirText, ok := strings.Cut(strings.TrimSpace(part), ":")
		field = strings.TrimSpace(field)
		dirText = strings.TrimSpace(dirText)
		if !ok || field == "" || (dirText != "1" && dirText != "-1") {
			return nil, fmt.Errorf("索引键写成字段:1 或 字段:-1，多项用逗号分开")
		}
		if err := validField(field); err != nil {
			return nil, err
		}
		if _, dup := seen[field]; dup {
			return nil, fmt.Errorf("索引字段重复")
		}
		seen[field] = struct{}{}
		dir := int32(1)
		if dirText == "-1" {
			dir = -1
		}
		out = append(out, indexField{field: field, dir: dir})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("请填写索引键")
	}
	return out, nil
}

func indexName(keys []indexField) string {
	parts := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		parts = append(parts, strings.ReplaceAll(key.field, ".", "_"), strconv.Itoa(int(key.dir)))
	}
	name := strings.Join(parts, "_")
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

func validDatabase(name string) error {
	if name == "" || name != strings.TrimSpace(name) || len(name) > 64 || strings.ContainsRune(name, 0) {
		return fmt.Errorf("库名无效")
	}
	if strings.ContainsAny(name, "/\\.\"$*<>:|?") {
		return fmt.Errorf("库名无效")
	}
	return nil
}

func validCollection(name string, write bool) error {
	if name == "" || name != strings.TrimSpace(name) || len(name) > 200 || strings.ContainsRune(name, 0) || strings.Contains(name, "$") {
		return fmt.Errorf("集合名无效")
	}
	if write && strings.HasPrefix(name, "system.") {
		return fmt.Errorf("不能操作系统集合")
	}
	return nil
}

func validIndexName(name string) error {
	if name == "" || name != strings.TrimSpace(name) || len(name) > 128 || strings.ContainsRune(name, 0) || strings.Contains(name, "$") {
		return fmt.Errorf("索引名无效")
	}
	return nil
}

func validField(name string) error {
	if name == "" || len(name) > 200 || strings.ContainsRune(name, 0) || strings.Contains(name, "$") {
		return fmt.Errorf("索引字段无效")
	}
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") {
		return fmt.Errorf("索引字段无效")
	}
	return nil
}

func protectedDatabase(name string) bool {
	switch strings.TrimSpace(name) {
	case "admin", "local", "config":
		return true
	default:
		return false
	}
}

func keyText(v any) string {
	doc, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	order, _ := doc["\x00"].([]string)
	if len(order) == 0 {
		for name := range doc {
			if name == "\x00" {
				continue
			}
			order = append(order, name)
		}
		sort.Strings(order)
	}
	parts := make([]string, 0, len(order))
	for _, name := range order {
		if name == "\x00" {
			continue
		}
		switch n := doc[name].(type) {
		case string:
			parts = append(parts, name+":"+n)
		case float64, int32, int64:
			parts = append(parts, fmt.Sprintf("%s:%d", name, int64(num(n))))
		default:
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, ", ")
}
