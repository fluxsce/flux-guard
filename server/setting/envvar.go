package setting

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"fluxguard/common/crypto"
)

const (
	// GroupEnvVars 租户级全局环境变量，供进程内按名称读取，也可展开 ${NAME}。
	GroupEnvVars = "envVars"

	// KindText 文本。
	KindText = "text"
	// KindNumber 数字，值必须能按十进制解析。
	KindNumber = "number"
	// KindBool 布尔，库存 true 或 false。
	KindBool = "bool"
	// KindSecret 密文。入库加密，查询只回掩码。
	KindSecret = "secret"

	envVarSecretMask  = "********"
	maxEnvVarNameLen  = 64
	maxEnvVarValueLen = 8192
	maxEnvVarNoteLen  = 256
	maxEnvVarItems    = 200
)

var (
	envVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	envVarRefPattern  = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::([^}]*))?\}`)
)

// EnvVar 一条全局环境变量。密文的 Value 在库中为 ENCY_ 密文。
type EnvVar struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Value string `json:"value"`
	Note  string `json:"note"`
}

// EnvVars 环境变量分组。
type EnvVars struct {
	Items []EnvVar `json:"items"`
}

// EnvVarInput 保存时的一条变量。OriginalName 用于改名时找回原密文。
type EnvVarInput struct {
	Name         string `json:"name"`
	OriginalName string `json:"originalName"`
	Kind         string `json:"kind"`
	Value        string `json:"value"`
	Note         string `json:"note"`
}

// EnvVarView 查询回显。密文不返回原文。
type EnvVarView struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Value    string `json:"value"`
	HasValue bool   `json:"hasValue"`
	Note     string `json:"note"`
}

type envCache struct {
	mu     sync.RWMutex
	values map[string]map[string]string
}

var envs = &envCache{values: map[string]map[string]string{}}

// DefaultEnvVars 返回空列表。
func DefaultEnvVars() EnvVars {
	return EnvVars{Items: []EnvVar{}}
}

// ParseEnvVars 解析分组 JSON。非法内容视为空列表。
func ParseEnvVars(content string) EnvVars {
	v := DefaultEnvVars()
	if strings.TrimSpace(content) == "" {
		return v
	}
	if err := json.Unmarshal([]byte(content), &v); err != nil {
		return DefaultEnvVars()
	}
	if v.Items == nil {
		v.Items = []EnvVar{}
	}
	return v
}

// NormalizeKind 把空类型收成文本，未知类型原样返回以便校验拒绝。
func NormalizeKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "", KindText:
		return KindText
	case KindNumber, KindBool, KindSecret:
		return kind
	default:
		return strings.TrimSpace(kind)
	}
}

// ValidateEnvVarName 校验变量名：字母或下划线开头，仅字母、数字和下划线。
func ValidateEnvVarName(name string) error {
	if name == "" {
		return fmt.Errorf("环境变量名不能为空")
	}
	if len(name) > maxEnvVarNameLen {
		return fmt.Errorf("环境变量名最长 %d 个字符", maxEnvVarNameLen)
	}
	if !envVarNamePattern.MatchString(name) {
		return fmt.Errorf("环境变量名须以字母或下划线开头，仅含字母、数字和下划线")
	}
	return nil
}

// MergeEnvVars 用提交内容覆盖已有变量。密文留空或掩码时保留原值。
func MergeEnvVars(inputs []EnvVarInput, prev EnvVars) (EnvVars, error) {
	if len(inputs) > maxEnvVarItems {
		return EnvVars{}, fmt.Errorf("环境变量最多 %d 条", maxEnvVarItems)
	}
	stored := map[string]EnvVar{}
	for _, item := range prev.Items {
		stored[item.Name] = item
	}
	seen := map[string]struct{}{}
	items := make([]EnvVar, 0, len(inputs))
	for _, in := range inputs {
		item := EnvVar{
			Name:  strings.TrimSpace(in.Name),
			Kind:  NormalizeKind(in.Kind),
			Value: in.Value,
			Note:  strings.TrimSpace(in.Note),
		}
		if err := ValidateEnvVarName(item.Name); err != nil {
			return EnvVars{}, err
		}
		if _, ok := seen[item.Name]; ok {
			return EnvVars{}, fmt.Errorf("环境变量名重复: %s", item.Name)
		}
		seen[item.Name] = struct{}{}
		if len(item.Note) > maxEnvVarNoteLen {
			return EnvVars{}, fmt.Errorf("环境变量 %s 的备注过长", item.Name)
		}
		key := strings.TrimSpace(in.OriginalName)
		if key == "" {
			key = item.Name
		}
		old, hasOld := stored[key]
		value, err := prepareEnvValue(item, old, hasOld)
		if err != nil {
			return EnvVars{}, err
		}
		item.Value = value
		if len(item.Value) > maxEnvVarValueLen {
			return EnvVars{}, fmt.Errorf("环境变量 %s 的值过长", item.Name)
		}
		items = append(items, item)
	}
	return EnvVars{Items: items}, nil
}

func prepareEnvValue(item EnvVar, old EnvVar, hasOld bool) (string, error) {
	switch item.Kind {
	case KindText:
		return item.Value, nil
	case KindNumber:
		raw := strings.TrimSpace(item.Value)
		if raw == "" {
			return "", fmt.Errorf("环境变量 %s 需要数字", item.Name)
		}
		if _, err := strconv.ParseFloat(raw, 64); err != nil {
			return "", fmt.Errorf("环境变量 %s 不是数字", item.Name)
		}
		return raw, nil
	case KindBool:
		switch strings.TrimSpace(strings.ToLower(item.Value)) {
		case "true", "1", "yes":
			return "true", nil
		case "false", "0", "no", "":
			return "false", nil
		default:
			return "", fmt.Errorf("环境变量 %s 须为布尔值", item.Name)
		}
	case KindSecret:
		raw := item.Value
		if raw == "" || raw == envVarSecretMask {
			if hasOld && old.Kind == KindSecret && old.Value != "" {
				return old.Value, nil
			}
			if raw == envVarSecretMask {
				return "", fmt.Errorf("环境变量 %s 需要填写密文", item.Name)
			}
			return "", nil
		}
		if crypto.IsEncrypted(raw) {
			return raw, nil
		}
		sealed, err := crypto.Encrypt("", raw)
		if err != nil {
			return "", fmt.Errorf("环境变量 %s 加密失败", item.Name)
		}
		return sealed, nil
	default:
		return "", fmt.Errorf("环境变量 %s 的类型无效", item.Name)
	}
}

// EncodeEnvVars 把变量列表写成入库 JSON。
func EncodeEnvVars(v EnvVars) (string, error) {
	if v.Items == nil {
		v.Items = []EnvVar{}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// MaskEnvVars 生成查询回显。密文只给掩码。
func MaskEnvVars(v EnvVars) []EnvVarView {
	views := make([]EnvVarView, 0, len(v.Items))
	for _, item := range v.Items {
		view := EnvVarView{
			Name:     item.Name,
			Kind:     NormalizeKind(item.Kind),
			Note:     item.Note,
			HasValue: strings.TrimSpace(item.Value) != "",
		}
		if view.Kind == KindSecret {
			if view.HasValue {
				view.Value = envVarSecretMask
			}
		} else {
			view.Value = decodeEnvValue(item.Value)
			view.HasValue = view.Value != ""
		}
		views = append(views, view)
	}
	return views
}

// PutEnvVars 写入进程缓存。密文在这里解密，供全局读取。
func PutEnvVars(tenantID string, v EnvVars) {
	tenantID = tenantKey(tenantID)
	resolved := map[string]string{}
	for _, item := range v.Items {
		if item.Name == "" {
			continue
		}
		resolved[item.Name] = decodeEnvValue(item.Value)
	}
	envs.mu.Lock()
	envs.values[tenantID] = resolved
	envs.mu.Unlock()
}

// GetEnv 按名称读取租户变量的明文。不存在时 ok 为 false。
func GetEnv(tenantID, name string) (string, bool) {
	if name == "" {
		return "", false
	}
	tenantID = tenantKey(tenantID)
	envs.mu.RLock()
	defer envs.mu.RUnlock()
	vars, ok := envs.values[tenantID]
	if !ok {
		return "", false
	}
	val, ok := vars[name]
	return val, ok
}

// Expand 把 ${NAME} 或 ${NAME:默认值} 换成租户变量。未配置且无默认值时换成空串。只展开一层。
func Expand(tenantID, text string) string {
	if text == "" || !strings.Contains(text, "${") {
		return text
	}
	return envVarRefPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := envVarRefPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return ""
		}
		if val, ok := GetEnv(tenantID, parts[1]); ok {
			return val
		}
		if len(parts) >= 3 {
			return parts[2]
		}
		return ""
	})
}

func decodeEnvValue(stored string) string {
	if stored == "" {
		return ""
	}
	plain, err := crypto.Decrypt("", stored)
	if err != nil {
		return ""
	}
	return plain
}
