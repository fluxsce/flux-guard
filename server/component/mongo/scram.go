package mongo

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"net"
	"strconv"
	"strings"
	"unicode/utf8"

	"fluxguard/server/component/conn"
	"fluxguard/server/component/profile"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/text/unicode/norm"
)

// authPlan 是这一次拨号要用的认证。mechanism 为空表示不认证。
type authPlan struct {
	mechanism string
	user      string
	secret    string
	database  string
	tls       bool
}

func authMode(target conn.Open) (authPlan, error) {
	deploy, auth := profile.Split(target.OptionText)
	plan := authPlan{tls: deploy == "tls", user: target.UserName, secret: target.Secret}
	switch auth {
	case "", "none":
		return plan, nil
	case "password", "scram256":
		// 早期只保存了「用户名口令」，按 MongoDB 4.0 起的默认机制处理。
		plan.mechanism = "SCRAM-SHA-256"
		plan.database = dbOr(target.DatabaseName, "admin")
	case "scram1":
		plan.mechanism = "SCRAM-SHA-1"
		plan.database = dbOr(target.DatabaseName, "admin")
	case "plain":
		plan.mechanism = "PLAIN"
		plan.database = dbOr(target.DatabaseName, "$external")
	case "x509":
		plan.mechanism = "MONGODB-X509"
		plan.tls = true
		plan.database = "$external"
	case "aws":
		plan.mechanism = "MONGODB-AWS"
		plan.database = "$external"
	case "oidc":
		plan.mechanism = "MONGODB-OIDC"
		plan.database = "$external"
	default:
		return authPlan{}, fmt.Errorf("不支持的 MongoDB 认证方式")
	}
	if err := plan.ready(); err != nil {
		return authPlan{}, err
	}
	return plan, nil
}

func dbOr(name, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback
	}
	return name
}

func (p authPlan) ready() error {
	switch p.mechanism {
	case "SCRAM-SHA-256", "SCRAM-SHA-1":
		if strings.TrimSpace(p.user) == "" || p.secret == "" {
			return fmt.Errorf("MongoDB 认证需要用户名和口令")
		}
	case "PLAIN":
		if strings.TrimSpace(p.user) == "" || p.secret == "" {
			return fmt.Errorf("LDAP 需要用户名和口令")
		}
	case "MONGODB-X509":
		if strings.TrimSpace(p.secret) == "" {
			return fmt.Errorf("请填写客户端证书和私钥")
		}
	case "MONGODB-AWS":
		if strings.TrimSpace(p.user) == "" || strings.TrimSpace(p.secret) == "" {
			return fmt.Errorf("AWS 认证需要访问密钥编号和访问密钥")
		}
	case "MONGODB-OIDC":
		if strings.TrimSpace(p.secret) == "" {
			return fmt.Errorf("请填写访问令牌")
		}
	}
	return nil
}

func authenticateSCRAM(raw net.Conn, plan authPlan) error {
	user, password, err := mongoScramSecret(plan.mechanism, plan.user, plan.secret)
	if err != nil {
		return err
	}
	client, err := newScram(plan.mechanism, user, password, "")
	if err != nil {
		return err
	}
	resp, err := call(raw, bsonDoc(
		bsonInt32("saslStart", 1),
		bsonString("mechanism", plan.mechanism),
		bsonBinary("payload", []byte(client.first())),
		bsonString("$db", plan.database),
	))
	if err != nil {
		return err
	}
	var conv int64
	for step := 0; step < 4; step++ {
		if err := mongoCmdErr(resp); err != nil {
			return err
		}
		if step == 0 {
			id, ok := intOf(dig(resp, "conversationId"))
			if !ok {
				return fmt.Errorf("MongoDB 认证应答无效")
			}
			conv = id
		}
		payload := bytesOf(dig(resp, "payload"))
		done := truthy(dig(resp, "done"))
		if !client.sent {
			msg, err := client.final(string(payload))
			if err != nil {
				return err
			}
			resp, err = saslContinue(raw, plan.database, conv, []byte(msg))
			if err != nil {
				return err
			}
			continue
		}
		if len(payload) > 0 {
			if err := client.verify(string(payload)); err != nil {
				return err
			}
		} else if !client.checked {
			return fmt.Errorf("MongoDB 认证应答不完整")
		}
		if done {
			if !client.checked {
				return fmt.Errorf("MongoDB 认证应答不完整")
			}
			return nil
		}
		resp, err = saslContinue(raw, plan.database, conv, nil)
		if err != nil {
			return err
		}
	}
	return fmt.Errorf("MongoDB 认证未完成")
}

func saslContinue(raw net.Conn, database string, conv int64, payload []byte) (map[string]any, error) {
	return call(raw, bsonDoc(
		bsonInt32("saslContinue", 1),
		bsonNumber("conversationId", conv),
		bsonBinary("payload", payload),
		bsonString("$db", database),
	))
}

func mongoCmdErr(doc map[string]any) error {
	if num(dig(doc, "ok")) >= 1 {
		return nil
	}
	msg := text(dig(doc, "errmsg"))
	if msg == "" {
		return fmt.Errorf("MongoDB 认证失败")
	}
	return fmt.Errorf("MongoDB 认证失败：%s", msg)
}

// mongoScramSecret 按机制准备 SCRAM 口令。
// SHA-256 用 SASLprep 之后的口令。SHA-1 用 hex(MD5(用户名 + ":mongo:" + 口令))，摘要前不做 SASLprep。
func mongoScramSecret(mechanism, user, password string) (string, string, error) {
	if user == "" || password == "" {
		return "", "", fmt.Errorf("MongoDB 认证需要用户名和口令")
	}
	saslUser, err := saslprep(user)
	if err != nil {
		return "", "", err
	}
	if saslUser == "" {
		return "", "", fmt.Errorf("MongoDB 认证需要用户名和口令")
	}
	if mechanism == "SCRAM-SHA-1" {
		sum := md5.Sum([]byte(user + ":mongo:" + password))
		return saslUser, hex.EncodeToString(sum[:]), nil
	}
	saslPass, err := saslprep(password)
	if err != nil {
		return "", "", err
	}
	return saslUser, saslPass, nil
}

// scramClient 完成一轮 SCRAM 客户端计算。口令必须已经按机制准备好。
type scramClient struct {
	hash      func() hash.Hash
	user      string
	password  string
	nonce     string
	bare      string
	serverSig []byte
	sent      bool
	checked   bool
}

func newScram(mechanism, user, password, nonce string) (*scramClient, error) {
	var h func() hash.Hash
	switch mechanism {
	case "SCRAM-SHA-256":
		h = sha256.New
	case "SCRAM-SHA-1":
		h = sha1.New
	default:
		return nil, fmt.Errorf("不支持的 MongoDB 认证方式")
	}
	if nonce == "" {
		var buf [18]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return nil, err
		}
		nonce = base64.RawStdEncoding.EncodeToString(buf[:])
	}
	return &scramClient{hash: h, user: user, password: password, nonce: nonce}, nil
}

func (c *scramClient) first() string {
	c.bare = "n=" + saslName(c.user) + ",r=" + c.nonce
	return "n,," + c.bare
}

func (c *scramClient) final(serverFirst string) (string, error) {
	attrs, err := scramAttrs(serverFirst)
	if err != nil {
		return "", err
	}
	full := attrs["r"]
	if full == "" || !strings.HasPrefix(full, c.nonce) {
		return "", fmt.Errorf("MongoDB 认证应答无效")
	}
	salt, err := base64.StdEncoding.DecodeString(attrs["s"])
	if err != nil || len(salt) == 0 {
		return "", fmt.Errorf("MongoDB 认证应答无效")
	}
	iter, err := strconv.Atoi(attrs["i"])
	if err != nil {
		return "", fmt.Errorf("MongoDB 认证应答无效")
	}
	without := "c=biws,r=" + full
	authMsg := c.bare + "," + serverFirst + "," + without
	proof, sig, err := scramProof(c.hash, c.password, salt, iter, authMsg)
	if err != nil {
		return "", err
	}
	c.serverSig = sig
	c.sent = true
	return without + ",p=" + base64.StdEncoding.EncodeToString(proof), nil
}

func (c *scramClient) verify(serverFinal string) error {
	if !c.sent || len(c.serverSig) == 0 {
		return fmt.Errorf("MongoDB 认证应答无效")
	}
	attrs, err := scramAttrs(serverFinal)
	if err != nil {
		return err
	}
	if msg := attrs["e"]; msg != "" {
		return fmt.Errorf("MongoDB 认证失败：%s", msg)
	}
	got, err := base64.StdEncoding.DecodeString(attrs["v"])
	if err != nil || subtle.ConstantTimeCompare(got, c.serverSig) != 1 {
		return fmt.Errorf("MongoDB 认证失败")
	}
	c.checked = true
	return nil
}

func scramProof(h func() hash.Hash, password string, salt []byte, iter int, authMessage string) ([]byte, []byte, error) {
	if iter < 4096 || iter > 100000 {
		return nil, nil, fmt.Errorf("MongoDB 认证迭代次数无效")
	}
	mac := func(key, msg []byte) []byte {
		m := hmac.New(h, key)
		_, _ = m.Write(msg)
		return m.Sum(nil)
	}
	salted := pbkdf2.Key([]byte(password), salt, iter, h().Size(), h)
	clientKey := mac(salted, []byte("Client Key"))
	sum := h()
	_, _ = sum.Write(clientKey)
	stored := sum.Sum(nil)
	clientSig := mac(stored, []byte(authMessage))
	if len(clientKey) != len(clientSig) {
		return nil, nil, fmt.Errorf("MongoDB 认证失败")
	}
	proof := make([]byte, len(clientKey))
	for i := range clientKey {
		proof[i] = clientKey[i] ^ clientSig[i]
	}
	serverKey := mac(salted, []byte("Server Key"))
	return proof, mac(serverKey, []byte(authMessage)), nil
}

func scramAttrs(msg string) (map[string]string, error) {
	if msg == "" {
		return nil, fmt.Errorf("MongoDB 认证应答无效")
	}
	out := map[string]string{}
	for _, part := range strings.Split(msg, ",") {
		key, val, ok := strings.Cut(part, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("MongoDB 认证应答无效")
		}
		out[key] = val
	}
	return out, nil
}

func saslName(user string) string {
	user = strings.ReplaceAll(user, "=", "=3D")
	return strings.ReplaceAll(user, ",", "=2C")
}

// saslprep 做 RFC 4013 的映射和 NFKC，并拒绝控制字符。
// 不检查双向文本规则。监控账号使用可打印字符即可。
func saslprep(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", fmt.Errorf("MongoDB 用户名或口令无效")
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case saslNothing(r):
			continue
		case saslSpace(r):
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	out := norm.NFKC.String(b.String())
	for _, r := range out {
		if r < 0x20 || r == 0x7F || (r >= 0x80 && r <= 0x9F) || r == 0xFFFD {
			return "", fmt.Errorf("MongoDB 用户名或口令含不允许的字符")
		}
	}
	return out, nil
}

func saslNothing(r rune) bool {
	switch r {
	case 0x00AD, 0x034F, 0x1806, 0x180B, 0x180C, 0x180D, 0x200B, 0x200C, 0x200D, 0x2060, 0xFEFF:
		return true
	default:
		return r >= 0xFE00 && r <= 0xFE0F
	}
}

func saslSpace(r rune) bool {
	switch r {
	case 0x00A0, 0x1680, 0x202F, 0x205F, 0x3000:
		return true
	default:
		return r >= 0x2000 && r <= 0x200A
	}
}
