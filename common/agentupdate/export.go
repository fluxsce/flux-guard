package agentupdate

// Untar 把 gzip 压缩的 tar 解到 dest，拒绝跳出 dest 的路径。
func Untar(src, dest string) error { return untar(src, dest) }

// PayloadRoot 在解压结果只有一个目录时返回该目录，否则返回 stage 本身。
func PayloadRoot(stage string) (string, error) { return payloadRoot(stage) }

// MergeTree 把安装包合进目标目录。跳过同名二进制、已有 configs/*.yaml，以及 data、logs。
func MergeTree(src, dest, binBase string) error { return mergeTree(src, dest, binBase) }

// FindBinary 在目录根查找程序。Windows 安装包里的名字带 .exe。
func FindBinary(root, base string) string { return findBinary(root, base) }

// CopyFile 覆盖复制单个文件，并保留源文件权限。
func CopyFile(src, dest string) error { return copyFile(src, dest) }
