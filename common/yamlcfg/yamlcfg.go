// Package yamlcfg 用 viper 读取 YAML 配置。文件不存在时保持调用方填好的默认值。
package yamlcfg

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Read 把 path 的 YAML 解到 dst。path 为空或文件不存在时返回 nil。
// 环境变量前缀为 FLUX_GUARD，键中的点换成下划线，可覆盖文件里已经写出的键。
func Read(path string, dst any) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("FLUX_GUARD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	// Get 才会叠上环境变量；Unmarshal 只看已经写入 viper 的值。
	for _, key := range v.AllKeys() {
		v.Set(key, v.Get(key))
	}
	return v.Unmarshal(dst)
}
