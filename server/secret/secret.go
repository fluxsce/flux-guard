// Package secret 是服务器的口令与代理令牌摘要。
// 登录口令只做 bcrypt 单向哈希，不能从库里还原明文。
// 需要还原原文的值用 common/crypto，不要放进口令字段。
// 代理令牌本身是高熵随机串，只存 SHA-256。
package secret

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var errEmptyPassword = errors.New("密码不能为空")

// HashPassword 生成 bcrypt 口令哈希。空口令拒绝。
func HashPassword(plain string) (string, error) {
	if plain == "" {
		return "", errEmptyPassword
	}
	sum, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(sum), nil
}

// MatchPassword 只用 bcrypt 校验口令。可逆密文和明文都不接受。
func MatchPassword(stored, plain string) bool {
	if stored == "" || plain == "" || !isBcryptHash(stored) {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(stored), []byte(plain)) == nil
}

func isBcryptHash(stored string) bool {
	return strings.HasPrefix(stored, "$2a$") ||
		strings.HasPrefix(stored, "$2b$") ||
		strings.HasPrefix(stored, "$2y$")
}

// HashToken 计算代理令牌的 SHA-256 十六进制摘要。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// MatchToken 以常数时间比较令牌摘要。
func MatchToken(hash, token string) bool {
	got := HashToken(token)
	if len(got) != len(hash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(hash)) == 1
}
