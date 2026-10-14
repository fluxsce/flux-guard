// Package errcode 定义带稳定错误码的业务错误。
// 错误码给前端做多语言，Text 是默认中文。
package errcode

// Error 是可以原样返回给管理端的业务错误。
// 同一指针用 errors.Is 比较；Code 在失败响应里原样输出。
type Error struct {
	id   string
	text string
}

// New 创建一条带错误码的业务错误。code 稳定不变，text 是默认中文。
func New(code, text string) *Error {
	return &Error{id: code, text: text}
}

// Error 返回默认中文。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.text
}

// Code 返回稳定错误码。
func (e *Error) Code() string {
	if e == nil {
		return ""
	}
	return e.id
}
