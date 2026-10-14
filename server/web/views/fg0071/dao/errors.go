package dao

import "errors"

// ErrMissing 表示应用不存在。
var ErrMissing = errors.New("找不到这个应用")
