package mongo

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

func bsonFromJSON(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return bsonDoc(), nil
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, fmt.Errorf("查询条件不是 JSON")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("查询条件不是 JSON")
	}
	doc, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("查询条件要写成 JSON 对象")
	}
	return encodeDoc(doc)
}

func encodeDoc(doc map[string]any) ([]byte, error) {
	elems := make([][]byte, 0, len(doc))
	for key, value := range doc {
		if strings.HasPrefix(key, "$") {
			if _, ok := queryOps[key]; !ok {
				return nil, fmt.Errorf("不支持这个查询运算符")
			}
		}
		elem, err := encodeElem(key, value)
		if err != nil {
			return nil, err
		}
		elems = append(elems, elem)
	}
	return bsonDoc(elems...), nil
}

func encodeElem(name string, value any) ([]byte, error) {
	switch item := value.(type) {
	case nil:
		return append([]byte{0x0A}, cstring(name)...), nil
	case bool:
		return bsonBool(name, item), nil
	case string:
		return bsonString(name, item), nil
	case json.Number:
		return encodeNumber(name, item)
	case map[string]any:
		if len(item) == 1 {
			if oid, ok := item["$oid"].(string); ok {
				return bsonObjectID(name, oid)
			}
			if when, ok := item["$date"].(string); ok {
				return bsonDate(name, when)
			}
		}
		doc, err := encodeDoc(item)
		if err != nil {
			return nil, err
		}
		return bsonSub(name, doc), nil
	case []any:
		body, err := encodeArray(item)
		if err != nil {
			return nil, err
		}
		return append(append([]byte{0x04}, cstring(name)...), body...), nil
	default:
		return nil, fmt.Errorf("查询条件含不支持的类型")
	}
}

func encodeArray(values []any) ([]byte, error) {
	elems := make([][]byte, 0, len(values))
	for i, value := range values {
		elem, err := encodeElem(strconv.Itoa(i), value)
		if err != nil {
			return nil, err
		}
		elems = append(elems, elem)
	}
	return bsonDoc(elems...), nil
}

func encodeNumber(name string, value json.Number) ([]byte, error) {
	text := value.String()
	if strings.ContainsAny(text, ".eE") {
		n, err := value.Float64()
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, fmt.Errorf("查询条件里的数字无效")
		}
		return bsonDouble(name, n), nil
	}
	n, err := value.Int64()
	if err != nil {
		return nil, fmt.Errorf("查询条件里的数字无效")
	}
	return bsonNumber(name, n), nil
}

func bsonObjectID(name, hexText string) ([]byte, error) {
	raw, err := hex.DecodeString(hexText)
	if err != nil || len(raw) != 12 {
		return nil, fmt.Errorf("对象编号无效")
	}
	out := append([]byte{0x07}, cstring(name)...)
	return append(out, raw...), nil
}

func bsonDate(name, text string) ([]byte, error) {
	when, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return nil, fmt.Errorf("时间无效")
	}
	out := append([]byte{0x09}, cstring(name)...)
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], uint64(when.UnixMilli()))
	return append(out, n[:]...), nil
}

var queryOps = map[string]struct{}{
	"$eq": {}, "$gt": {}, "$gte": {}, "$lt": {}, "$lte": {}, "$ne": {},
	"$in": {}, "$nin": {}, "$and": {}, "$or": {}, "$nor": {}, "$not": {},
	"$exists": {}, "$regex": {}, "$options": {}, "$elemMatch": {}, "$size": {},
	"$all": {}, "$mod": {}, "$oid": {}, "$date": {},
}
