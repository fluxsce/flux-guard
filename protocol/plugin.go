package protocol

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	// ExtVersion 是外置插件信封的版本。请求和应答都必须是这一版。
	ExtVersion = 1
	// MaxExtFrame 是一帧 JSON 的上限。
	MaxExtFrame = 1 << 20

	extFrameHeader = 4

	// ExtCodeInternal 是未分类失败。
	ExtCodeInternal = "internal"
	// ExtCodeMethodNotFound 表示这次调用的方法不在该插件的清单里。
	ExtCodeMethodNotFound = "method_not_found"
	// ExtCodeInvalidRequest 表示信封无法解析，或调用号对不上。
	ExtCodeInvalidRequest = "invalid_request"
	// ExtCodeInvalidParams 表示参数缺失或非法。
	ExtCodeInvalidParams = "invalid_params"
	// ExtCodeCancelled 表示调用方取消。
	ExtCodeCancelled = "cancelled"
	// ExtCodeUnavailable 表示插件进程不可用。
	ExtCodeUnavailable = "unavailable"
	// ExtCodeTimeout 表示这一次调用超时。
	ExtCodeTimeout = "timeout"
	// ExtCodeLost 表示管道已断开。
	ExtCodeLost = "lost"
)

var (
	extID = regexp.MustCompile(`^[A-Za-z0-9]{8,32}$`)

	// ErrExtFrameTooLarge 表示对端声明的帧超过 MaxExtFrame。调用方应停掉该进程，避免下一帧读错位。
	ErrExtFrameTooLarge = errors.New("插件应答过大")
)

// ExtRequest 是中央写给外置插件的一条调用。
// Method 和 Params 的取值来自 plugins/manifests 里该插件的清单，不在本文件登记。
// 每笔调用单独连到插件监听的本机管道。跟踪靠 ID 与 TraceID。
type ExtRequest struct {
	V       int             `json:"v"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      string          `json:"id"`
	TraceID string          `json:"traceId"`
}

// ExtResponse 是外置插件的一条应答。
// Result 是业务 JSON 再编码成的字符串，方法增减时帧格式不变。
// 失败时 OK 为 false，Error 是给人看的说明，ErrorCode 是稳定错误码。
type ExtResponse struct {
	V         int    `json:"v"`
	ID        string `json:"id"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"errorCode,omitempty"`
	TraceID   string `json:"traceId"`
	Result    string `json:"result"`
}

// ExtFailure 是插件返回的失败，或中央在同一次跟踪上记下的传输失败。
// Error 返回给人看的说明。Code 供日志和后续分支使用，不要靠文案判断。
type ExtFailure struct {
	Code    string
	Message string
	TraceID string
}

// Error 返回给人看的说明。没有说明时退回错误码。
func (e *ExtFailure) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

// ValidExtID 判断调用号或跟踪号的格式。两者使用同一套字符规则。
func ValidExtID(id string) bool {
	return extID.MatchString(id)
}

// EncodeExtResult 把业务对象编成信封里的 result 字符串。
func EncodeExtResult(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// DecodeExtResult 把信封里的 result 字符串解成业务对象。空串或非法 JSON 视为应答无效。
func DecodeExtResult(result string, dest any) error {
	if dest == nil {
		return nil
	}
	if strings.TrimSpace(result) == "" || !json.Valid([]byte(result)) {
		return fmt.Errorf("插件应答无效")
	}
	if err := json.Unmarshal([]byte(result), dest); err != nil {
		return fmt.Errorf("插件应答无效")
	}
	return nil
}

// CheckExtResponse 核对应答的版本、调用号和跟踪号。对不上就不能把结果算到这次调用上。
func CheckExtResponse(req ExtRequest, resp ExtResponse) error {
	if resp.V != ExtVersion || resp.ID != req.ID || resp.TraceID != req.TraceID {
		return &ExtFailure{Code: ExtCodeInvalidRequest, Message: "插件应答无效", TraceID: req.TraceID}
	}
	if !ValidExtID(resp.ID) || !ValidExtID(resp.TraceID) {
		return &ExtFailure{Code: ExtCodeInvalidRequest, Message: "插件应答无效", TraceID: req.TraceID}
	}
	return nil
}

// WriteExtFrame 写入一帧：4 字节小端长度，随后是 UTF-8 JSON。
func WriteExtFrame(w io.Writer, payload []byte) error {
	if len(payload) > MaxExtFrame {
		return ErrExtFrameTooLarge
	}
	var header [extFrameHeader]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

// ReadExtFrame 读出一帧 JSON。声明的长度超过 MaxExtFrame 时不继续读正文。
func ReadExtFrame(r io.Reader) ([]byte, error) {
	var header [extFrameHeader]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(header[:])
	if n > MaxExtFrame {
		return nil, ErrExtFrameTooLarge
	}
	if n == 0 {
		return []byte{}, nil
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
