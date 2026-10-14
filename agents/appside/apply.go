package appside

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ApplyReport 是一次换包/升级落盘结果，供命令回执审计目标与备份路径。
type ApplyReport struct {
	TargetPath string
	BackupPath string
}

// ReplaceWar 用新包替换 webapps 里的同名 WAR，并清掉已展开的目录。
// 旧包先移到 backupDir 下该实例的目录，原路径不留 .bak。
func ReplaceWar(base, app, src, backupDir, instance string) (ApplyReport, error) {
	app = strings.TrimSpace(app)
	if app == "" || strings.Contains(app, "..") || strings.ContainsAny(app, `/\`) {
		return ApplyReport{}, fmt.Errorf("应用名无效")
	}
	webapps := filepath.Join(base, "webapps")
	if err := os.MkdirAll(webapps, 0o755); err != nil {
		return ApplyReport{}, err
	}
	dest := filepath.Join(webapps, app+".war")
	report := ApplyReport{TargetPath: dest}
	parked, err := parkBackup(backupDir, instance, app+".war", dest)
	if err != nil {
		return report, err
	}
	report.BackupPath = parked
	if err := copyFile(src, dest); err != nil {
		_ = restoreBackup(parked, dest)
		return report, err
	}
	exploded := filepath.Join(webapps, app)
	if err := os.RemoveAll(exploded); err != nil {
		return report, err
	}
	return report, nil
}

// ReplaceJar 替换配置里的那个 JAR。旧文件先移到备份目录。
func ReplaceJar(dest, src, backupDir, instance string) (ApplyReport, error) {
	if strings.TrimSpace(dest) == "" {
		return ApplyReport{}, fmt.Errorf("未配置 JAR 路径")
	}
	report := ApplyReport{TargetPath: dest}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return report, err
	}
	parked, err := parkBackup(backupDir, instance, filepath.Base(dest), dest)
	if err != nil {
		return report, err
	}
	report.BackupPath = parked
	if err := copyFile(src, dest); err != nil {
		_ = restoreBackup(parked, dest)
		return report, err
	}
	return report, nil
}

// ReplaceFile 把上传文件写进目录仓里的同名（或同应用名）文件，不做整目录解压。
// 优先覆盖目录里已有、AppCode 与 appName 相同的文件；没有则用 fileName。
func ReplaceFile(dir, appName, src, fileName, backupDir, instance string) (ApplyReport, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ApplyReport{}, fmt.Errorf("未配置文件目录")
	}
	name := resolveFileName(dir, appName, fileName)
	if name == "" {
		return ApplyReport{}, fmt.Errorf("文件名无效")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ApplyReport{}, err
	}
	target := filepath.Join(dir, name)
	report := ApplyReport{TargetPath: target}
	parked, err := parkBackup(backupDir, instance, name, target)
	if err != nil {
		return report, err
	}
	report.BackupPath = parked
	if err := copyFile(src, target); err != nil {
		_ = restoreBackup(parked, target)
		return report, err
	}
	return report, nil
}

// resolveFileName 选定目录仓里要覆盖的相对路径（可含嵌套目录）。
func resolveFileName(dir, appName, fileName string) string {
	appName = strings.ReplaceAll(strings.TrimSpace(appName), `\`, "/")
	upload := filepath.Base(strings.TrimSpace(fileName))
	if upload == "." || upload == ".." || strings.Contains(upload, "..") {
		upload = ""
	}
	if appName == "" || strings.Contains(appName, "..") {
		if upload == "" {
			return ""
		}
		return upload
	}
	exts := []string{".war", ".jar", ".zip"}
	if upload != "" {
		if e := filepath.Ext(upload); e != "" {
			exts = append([]string{e}, exts...)
		}
	}
	for _, ext := range exts {
		rel := filepath.FromSlash(appName) + ext
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			return rel
		}
	}
	var found string
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		got, _ := AppCode(entry.Name())
		if got != appName {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr == nil && rel != "" && !strings.HasPrefix(rel, "..") {
			found = rel
			return filepath.SkipAll
		}
		return nil
	})
	if found != "" {
		return found
	}
	ext := ".war"
	if upload != "" {
		if e := filepath.Ext(upload); e != "" {
			ext = e
		}
	}
	return filepath.FromSlash(appName) + ext
}

// ReplaceStatic 把文件放进静态目录。zip 则先把原目录移到备份目录，再解进原目录。
func ReplaceStatic(dir, src, fileName, backupDir, instance string) (ApplyReport, error) {
	if strings.TrimSpace(dir) == "" {
		return ApplyReport{}, fmt.Errorf("未配置静态目录")
	}
	if strings.EqualFold(filepath.Ext(fileName), ".zip") {
		report := ApplyReport{TargetPath: dir}
		parked, err := parkBackup(backupDir, instance, filepath.Base(dir), dir)
		if err != nil {
			return report, err
		}
		report.BackupPath = parked
		if err := unzip(src, dir); err != nil {
			_ = restoreBackup(parked, dir)
			return report, err
		}
		return report, nil
	}
	name := filepath.Base(fileName)
	if name == "" || name == "." || strings.Contains(name, "..") {
		return ApplyReport{}, fmt.Errorf("文件名无效")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ApplyReport{}, err
	}
	target := filepath.Join(dir, name)
	report := ApplyReport{TargetPath: target}
	parked, err := parkBackup(backupDir, instance, name, target)
	if err != nil {
		return report, err
	}
	report.BackupPath = parked
	if err := copyFile(src, target); err != nil {
		_ = restoreBackup(parked, target)
		return report, err
	}
	return report, nil
}

// parkBackup 把已有文件或目录移进备份目录。源不存在时不产生备份。
// 同一磁盘直接改名。跨磁盘时先复制再删掉源。
func parkBackup(backupDir, instance, label, src string) (string, error) {
	backupDir = strings.TrimSpace(backupDir)
	if backupDir == "" {
		return "", fmt.Errorf("未配置备份目录")
	}
	instance = strings.TrimSpace(instance)
	label = strings.TrimSpace(label)
	if instance == "" || strings.Contains(instance, "..") || strings.ContainsAny(instance, `/\`) {
		return "", fmt.Errorf("实例名无效")
	}
	if label == "" || label == "." || strings.Contains(label, "..") || strings.ContainsAny(label, `/\`) {
		return "", fmt.Errorf("备份名无效")
	}
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	dir := filepath.Join(backupDir, instance)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(label)
	stem := strings.TrimSuffix(label, ext)
	dest := filepath.Join(dir, stem+"-"+time.Now().UTC().Format("20060102-150405")+ext)
	for n := 0; ; n++ {
		try := dest
		if n > 0 {
			try = strings.TrimSuffix(dest, ext) + "-" + strconv.Itoa(n) + ext
		}
		if _, err := os.Stat(try); os.IsNotExist(err) {
			dest = try
			break
		}
		if n >= 20 {
			return "", fmt.Errorf("备份文件已存在")
		}
	}
	if err := os.Rename(src, dest); err == nil {
		return dest, nil
	}
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		if err := copyTree(src, dest); err != nil {
			_ = os.RemoveAll(dest)
			return "", err
		}
	} else if err := copyFile(src, dest); err != nil {
		_ = os.Remove(dest)
		return "", err
	}
	if err := os.RemoveAll(src); err != nil {
		return "", err
	}
	return dest, nil
}

func restoreBackup(parked, dest string) error {
	if parked == "" {
		return os.RemoveAll(dest)
	}
	_ = os.RemoveAll(dest)
	if err := os.Rename(parked, dest); err == nil {
		return nil
	}
	info, err := os.Stat(parked)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyTree(parked, dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return copyFile(parked, dest)
}

// Rollback 把 backupDir 下的备份移回目标路径。备份必须落在 backupDir 内。
// war 回滚后清掉已展开目录，与 ReplaceWar 一致。
func Rollback(backupDir, target, backup, kind, appName string) (ApplyReport, error) {
	target = strings.TrimSpace(target)
	backup = strings.TrimSpace(backup)
	if target == "" || backup == "" {
		return ApplyReport{}, fmt.Errorf("回滚缺少目标或备份路径")
	}
	if err := pathUnder(backupDir, backup); err != nil {
		return ApplyReport{}, err
	}
	report := ApplyReport{TargetPath: target, BackupPath: backup}
	if _, err := os.Stat(backup); err != nil {
		if os.IsNotExist(err) {
			return report, fmt.Errorf("备份不存在")
		}
		return report, err
	}
	if err := restoreBackup(backup, target); err != nil {
		return report, err
	}
	if kind == "war" {
		appName = strings.TrimSpace(appName)
		if appName != "" && !strings.Contains(appName, "..") && !strings.ContainsAny(appName, `/\`) {
			_ = os.RemoveAll(filepath.Join(filepath.Dir(target), appName))
		}
	}
	return report, nil
}

// pathUnder 要求 path 落在 root 目录内（含自身）。
func pathUnder(root, path string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return fmt.Errorf("未配置备份目录")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("备份路径不在备份目录内")
	}
	return nil
}

func copyTree(src, dest string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

// keepDirs 是运行时升级时要从旧目录拷回的实例数据。程序目录路径不变。
var keepDirs = []string{"conf", "logs", "webapps", "temp", "work"}

// UpgradeBase 在原目录升级运行时：先把整目录挪到 backup_dir，再解压到原路径，
// 并从备份里把 conf、logs、webapps、temp、work 拷回。部署路径不变。
func UpgradeBase(base, zipPath, backupDir, instance string) (ApplyReport, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return ApplyReport{}, fmt.Errorf("未配置运行时目录")
	}
	report := ApplyReport{TargetPath: base}
	parked, err := parkBackup(backupDir, instance, "runtime", base)
	if err != nil {
		return report, err
	}
	report.BackupPath = parked
	if err := unzip(zipPath, base); err != nil {
		_ = restoreBackup(parked, base)
		return report, err
	}
	if parked == "" {
		return report, nil
	}
	for _, name := range keepDirs {
		src := filepath.Join(parked, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dest := filepath.Join(base, name)
		if err := os.RemoveAll(dest); err != nil {
			_ = os.RemoveAll(base)
			_ = restoreBackup(parked, base)
			return report, err
		}
		if err := copyTree(src, dest); err != nil {
			_ = os.RemoveAll(base)
			_ = restoreBackup(parked, base)
			return report, err
		}
	}
	return report, nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func unzip(src, dest string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	root, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		target, err := safeJoin(root, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func safeJoin(root, name string) (string, error) {
	clean := filepath.Clean(name)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("压缩包路径无效")
	}
	target := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("压缩包路径无效")
	}
	return target, nil
}
