package shellrun

// terminal 是一块能改行列的屏幕。按键原样写入，输出按控制序列读出。
type terminal interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
	Resize(cols, rows int) error
}
