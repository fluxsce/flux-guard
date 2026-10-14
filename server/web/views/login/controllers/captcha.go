package controllers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"fluxguard/server/utils/const/errcode"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/login/models"
)

const (
	captchaTicketVersion = 1
	captchaNonceSize     = 16
	captchaMACSize       = sha256.Size
	captchaTicketSize    = 1 + 8 + captchaNonceSize + captchaMACSize
	captchaCodeLength    = 6
	captchaTTL           = limits.CaptchaTTL
	captchaClockSkew     = limits.CaptchaClockSkew
	captchaUsedPrefix    = "captcha:used:"
	captchaDevSecret     = "fluxguard-captcha-dev-secret"
)

var (
	// ErrCaptchaRequired 未提交验证码。
	ErrCaptchaRequired = errcode.New("auth.captcha.required", "验证码不能为空")
	// ErrCaptchaExpired 票缺失、损坏或已过期。对外统一该文案，避免帮助探测。
	ErrCaptchaExpired = errcode.New("auth.captcha.expired", "验证码不存在或已过期")
	// ErrCaptchaInvalid 用户输入与票不匹配。
	ErrCaptchaInvalid = errcode.New("auth.captcha.invalid", "验证码错误")
	// ErrCaptchaType 不支持的验证码类型。
	ErrCaptchaType = errcode.New("auth.captcha.type", "不支持的验证码类型")
)

// Captcha 图形验证码。HMAC 签名票里不含答案，校验时按 nonce 写入缓存核销。
type Captcha struct {
	secret []byte
	ttl    time.Duration
}

// NewCaptcha 创建验证码。已用票记在进程缓存里，到期后自动失效。
func NewCaptcha() *Captcha {
	return &Captcha{
		secret: []byte(captchaDevSecret),
		ttl:    captchaTTL,
	}
}

// Generate 生成图形验证码。只返回签名票、PNG 图和过期时间。
func (s *Captcha) Generate(_ context.Context, req *models.CaptchaRequest) (*models.CaptchaResponse, error) {
	if s == nil {
		s = NewCaptcha()
	}
	if req == nil {
		req = &models.CaptchaRequest{}
	}
	captchaType := req.Type
	if captchaType == "" {
		captchaType = "random"
	}
	if captchaType != "random" {
		return nil, fmt.Errorf("%w: %s", ErrCaptchaType, captchaType)
	}
	code, err := generateRandomDigits(captchaCodeLength)
	if err != nil {
		return nil, fmt.Errorf("生成验证码失败: %w", err)
	}
	expireAt := time.Now().Add(s.ttl)
	ticket, err := s.signTicket(code, expireAt)
	if err != nil {
		return nil, fmt.Errorf("签发验证码票失败: %w", err)
	}
	imageDataURI, err := renderCaptchaPNGDataURI(code)
	if err != nil {
		return nil, fmt.Errorf("绘制验证码图片失败: %w", err)
	}
	return &models.CaptchaResponse{
		CaptchaId: ticket,
		Image:     imageDataURI,
		ExpireAt:  expireAt.Unix(),
	}, nil
}

// Verify 校验用户输入并核销该票。结构合法且未过期的票首次校验即作废，无论答案对错。
func (s *Captcha) Verify(captchaId, code string) error {
	if s == nil {
		return ErrCaptchaExpired
	}
	if captchaId == "" || strings.TrimSpace(code) == "" {
		return ErrCaptchaRequired
	}
	raw, err := base64.RawURLEncoding.DecodeString(captchaId)
	if err != nil || len(raw) != captchaTicketSize || raw[0] != captchaTicketVersion {
		return ErrCaptchaExpired
	}
	expUnix := int64(binary.BigEndian.Uint64(raw[1:9]))
	if time.Now().Add(-captchaClockSkew).Unix() > expUnix {
		return ErrCaptchaExpired
	}
	nonce := raw[9 : 9+captchaNonceSize]
	if err := s.consumeTicket(nonce, expUnix); err != nil {
		return err
	}
	expectedMAC := raw[9+captchaNonceSize:]
	actualMAC := s.computeMAC(raw[0], expUnix, nonce, normalizeCaptchaCode(code))
	if !hmac.Equal(expectedMAC, actualMAC) {
		return ErrCaptchaInvalid
	}
	return nil
}

func (s *Captcha) consumeTicket(nonce []byte, expUnix int64) error {
	key := captchaUsedPrefix + hex.EncodeToString(nonce)
	expireAt := time.Unix(expUnix, 0).Add(captchaClockSkew)
	ttl := time.Until(expireAt)
	if ttl < time.Second {
		ttl = time.Second
	}
	ok, err := request.PutOnce(context.Background(), key, "1", ttl)
	if err != nil || !ok {
		return ErrCaptchaExpired
	}
	return nil
}

func (s *Captcha) signTicket(code string, expireAt time.Time) (string, error) {
	nonce := make([]byte, captchaNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	expUnix := expireAt.Unix()
	mac := s.computeMAC(captchaTicketVersion, expUnix, nonce, normalizeCaptchaCode(code))
	buf := make([]byte, captchaTicketSize)
	buf[0] = captchaTicketVersion
	binary.BigEndian.PutUint64(buf[1:9], uint64(expUnix))
	copy(buf[9:9+captchaNonceSize], nonce)
	copy(buf[9+captchaNonceSize:], mac)
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *Captcha) computeMAC(version byte, expUnix int64, nonce []byte, code string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte{version})
	var expBuf [8]byte
	binary.BigEndian.PutUint64(expBuf[:], uint64(expUnix))
	mac.Write(expBuf[:])
	mac.Write(nonce)
	mac.Write([]byte(code))
	return mac.Sum(nil)
}

func normalizeCaptchaCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func generateRandomDigits(length int) (string, error) {
	const digits = "0123456789"
	out := make([]byte, length)
	for i := 0; i < length; {
		var b [1]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		if b[0] >= 250 {
			continue
		}
		out[i] = digits[int(b[0])%10]
		i++
	}
	return string(out), nil
}
