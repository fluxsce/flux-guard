package validate

// ErrText 返回可直接展示给用户的错误。
func ErrText(text string) error {
	return &textError{text}
}

type textError struct{ text string }

func (e *textError) Error() string { return e.text }
