package mongo

import (
	"encoding/binary"
	"fmt"
	"math"
)

func bsonDoc(elems ...[]byte) []byte {
	body := []byte{}
	for _, elem := range elems {
		body = append(body, elem...)
	}
	size := 4 + len(body) + 1
	out := make([]byte, 4, size)
	binary.LittleEndian.PutUint32(out, uint32(size))
	out = append(out, body...)
	return append(out, 0)
}

func bsonInt32(name string, v int32) []byte {
	out := append([]byte{0x10}, cstring(name)...)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(v))
	return append(out, n[:]...)
}

func bsonInt64(name string, v int64) []byte {
	out := append([]byte{0x12}, cstring(name)...)
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], uint64(v))
	return append(out, n[:]...)
}

func bsonDouble(name string, v float64) []byte {
	out := append([]byte{0x01}, cstring(name)...)
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], math.Float64bits(v))
	return append(out, n[:]...)
}

func bsonString(name, value string) []byte {
	out := append([]byte{0x02}, cstring(name)...)
	raw := append([]byte(value), 0)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(raw)))
	out = append(out, n[:]...)
	return append(out, raw...)
}

func bsonBool(name string, v bool) []byte {
	bit := byte(0)
	if v {
		bit = 1
	}
	return append(append([]byte{0x08}, cstring(name)...), bit)
}

func bsonSub(name string, doc []byte) []byte {
	return append(append([]byte{0x03}, cstring(name)...), doc...)
}

func bsonBinary(name string, v []byte) []byte {
	out := append([]byte{0x05}, cstring(name)...)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(v)))
	out = append(out, n[:]...)
	out = append(out, 0)
	return append(out, v...)
}

func bsonNumber(name string, v int64) []byte {
	if v >= math.MinInt32 && v <= math.MaxInt32 {
		return bsonInt32(name, int32(v))
	}
	return bsonInt64(name, v)
}

func cstring(name string) []byte {
	return append([]byte(name), 0)
}

func decodeDoc(b []byte) (map[string]any, error) {
	if len(b) < 5 {
		return nil, fmt.Errorf("MongoDB 应答无效")
	}
	size := int(binary.LittleEndian.Uint32(b[:4]))
	if size < 5 || size > len(b) {
		return nil, fmt.Errorf("MongoDB 应答无效")
	}
	return decodeElems(b[4 : size-1])
}

func decodeElems(b []byte) (map[string]any, error) {
	out := map[string]any{}
	i := 0
	for i < len(b) {
		typ := b[i]
		i++
		name, n, err := readCString(b[i:])
		if err != nil {
			return nil, err
		}
		i += n
		val, used, err := decodeValue(typ, b[i:])
		if err != nil {
			return nil, err
		}
		out[name] = val
		i += used
	}
	return out, nil
}

func decodeValue(typ byte, b []byte) (any, int, error) {
	switch typ {
	case 0x01:
		if len(b) < 8 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(b[:8])), 8, nil
	case 0x02, 0x0D, 0x0E:
		n, err := skipString(b)
		if err != nil {
			return nil, 0, err
		}
		if typ != 0x02 {
			return nil, n, nil
		}
		size := int(binary.LittleEndian.Uint32(b[:4]))
		return string(b[4 : 4+size-1]), n, nil
	case 0x03, 0x04:
		if len(b) < 4 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		n := int(binary.LittleEndian.Uint32(b[:4]))
		if n < 5 || n > len(b) {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		doc, err := decodeElems(b[4 : n-1])
		return doc, n, err
	case 0x05:
		if len(b) < 5 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		n := int(binary.LittleEndian.Uint32(b[:4]))
		if n < 0 || 5+n > len(b) {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		raw := b[5 : 5+n]
		// 旧式二进制在数据前再带一段长度。SASL 载荷用的是普通二进制。
		if b[4] == 0x02 && len(raw) >= 4 {
			inner := int(binary.LittleEndian.Uint32(raw[:4]))
			if inner >= 0 && 4+inner <= len(raw) {
				raw = raw[4 : 4+inner]
			}
		}
		out := make([]byte, len(raw))
		copy(out, raw)
		return out, 5 + n, nil
	case 0x07:
		if len(b) < 12 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return nil, 12, nil
	case 0x08:
		if len(b) < 1 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return b[0] != 0, 1, nil
	case 0x09:
		if len(b) < 8 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return int64(binary.LittleEndian.Uint64(b[:8])), 8, nil
	case 0x0A, 0x06, 0xFF, 0x7F:
		return nil, 0, nil
	case 0x0B:
		n, err := skipCString(b)
		if err != nil {
			return nil, 0, err
		}
		m, err := skipCString(b[n:])
		if err != nil {
			return nil, 0, err
		}
		return nil, n + m, nil
	case 0x0C:
		n, err := skipString(b)
		if err != nil {
			return nil, 0, err
		}
		if n+12 > len(b) {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return nil, n + 12, nil
	case 0x0F:
		if len(b) < 4 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		n := int(binary.LittleEndian.Uint32(b[:4]))
		if n < 4 || n > len(b) {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return nil, n, nil
	case 0x10:
		if len(b) < 4 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return int32(binary.LittleEndian.Uint32(b[:4])), 4, nil
	case 0x11:
		if len(b) < 8 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return int64(binary.LittleEndian.Uint32(b[4:8])), 8, nil
	case 0x12:
		if len(b) < 8 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return int64(binary.LittleEndian.Uint64(b[:8])), 8, nil
	case 0x13:
		if len(b) < 16 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return nil, 16, nil
	default:
		return nil, 0, fmt.Errorf("MongoDB 应答含未识别类型")
	}
}

func skipString(b []byte) (int, error) {
	if len(b) < 4 {
		return 0, fmt.Errorf("MongoDB 应答无效")
	}
	n := int(binary.LittleEndian.Uint32(b[:4]))
	if n < 1 || 4+n > len(b) {
		return 0, fmt.Errorf("MongoDB 应答无效")
	}
	return 4 + n, nil
}

func skipCString(b []byte) (int, error) {
	for i, c := range b {
		if c == 0 {
			return i + 1, nil
		}
	}
	return 0, fmt.Errorf("MongoDB 应答无效")
}

func readCString(b []byte) (string, int, error) {
	n, err := skipCString(b)
	if err != nil {
		return "", 0, err
	}
	return string(b[:n-1]), n, nil
}

func dig(v any, path ...string) any {
	cur, ok := lookup(v, path...)
	if !ok {
		return nil
	}
	return cur
}

func lookup(v any, path ...string) (any, bool) {
	cur := v
	for _, key := range path {
		doc, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := doc[key]
		if !ok || next == nil {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func text(v any) string {
	s, _ := v.(string)
	return s
}

func intOf(v any) (int64, bool) {
	switch n := v.(type) {
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

func bytesOf(v any) []byte {
	b, _ := v.([]byte)
	return b
}

func truthy(v any) bool {
	b, ok := v.(bool)
	return ok && b
}
