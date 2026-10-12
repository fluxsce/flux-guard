package mongo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"time"
)

// mongoTestRoots 只给测试替换信任的证书。正式拨号保持空，用系统信任库。
var mongoTestRoots *x509.CertPool

func authenticate(raw net.Conn, plan authPlan) error {
	switch plan.mechanism {
	case "SCRAM-SHA-1", "SCRAM-SHA-256":
		return authenticateSCRAM(raw, plan)
	case "PLAIN":
		payload, err := plainPayload(plan.user, plan.secret)
		if err != nil {
			return err
		}
		return saslDone(raw, plan.database, "PLAIN", payload)
	case "MONGODB-X509":
		return authenticateX509(raw, plan.user)
	case "MONGODB-AWS":
		return authenticateAWS(raw, plan)
	case "MONGODB-OIDC":
		token := strings.TrimSpace(plan.secret)
		if token == "" {
			return fmt.Errorf("请填写访问令牌")
		}
		return saslDone(raw, plan.database, "MONGODB-OIDC", bsonDoc(bsonString("jwt", token)))
	default:
		return fmt.Errorf("不支持的 MongoDB 认证方式")
	}
}

func dialMongo(ctx context.Context, addr string, plan authPlan) (net.Conn, error) {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	if !plan.tls {
		return raw, nil
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		raw.Close()
		return nil, err
	}
	cfg, err := mongoTLS(host, plan)
	if err != nil {
		raw.Close()
		return nil, err
	}
	secure := tls.Client(raw, cfg)
	if err := secure.HandshakeContext(ctx); err != nil {
		secure.Close()
		return nil, err
	}
	return secure, nil
}

func mongoTLS(serverName string, plan authPlan) (*tls.Config, error) {
	cfg := &tls.Config{
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
		RootCAs:    mongoTestRoots,
	}
	if plan.mechanism != "MONGODB-X509" {
		return cfg, nil
	}
	if !strings.Contains(plan.secret, "CERTIFICATE") || !strings.Contains(plan.secret, "PRIVATE KEY") {
		return nil, fmt.Errorf("请填写客户端证书和私钥")
	}
	cert, err := tls.X509KeyPair([]byte(plan.secret), []byte(plan.secret))
	if err != nil {
		return nil, fmt.Errorf("客户端证书无效")
	}
	cfg.Certificates = []tls.Certificate{cert}
	return cfg, nil
}

func saslDone(raw net.Conn, database, mechanism string, payload []byte) error {
	resp, err := call(raw, bsonDoc(
		bsonInt32("saslStart", 1),
		bsonString("mechanism", mechanism),
		bsonBinary("payload", payload),
		bsonString("$db", database),
	))
	if err != nil {
		return err
	}
	if err := mongoCmdErr(resp); err != nil {
		return err
	}
	if !truthy(dig(resp, "done")) {
		return fmt.Errorf("MongoDB 认证未完成")
	}
	return nil
}

func plainPayload(user, password string) ([]byte, error) {
	user, err := saslprep(user)
	if err != nil {
		return nil, err
	}
	password, err = saslprep(password)
	if err != nil {
		return nil, err
	}
	if user == "" || password == "" || strings.ContainsRune(user, 0) || strings.ContainsRune(password, 0) {
		return nil, fmt.Errorf("LDAP 需要用户名和口令")
	}
	out := make([]byte, 0, 2+len(user)+len(password))
	out = append(out, 0)
	out = append(out, user...)
	out = append(out, 0)
	out = append(out, password...)
	return out, nil
}

func authenticateX509(raw net.Conn, user string) error {
	elems := [][]byte{
		bsonInt32("authenticate", 1),
		bsonString("mechanism", "MONGODB-X509"),
	}
	if strings.TrimSpace(user) != "" {
		elems = append(elems, bsonString("user", strings.TrimSpace(user)))
	}
	elems = append(elems, bsonString("$db", "$external"))
	resp, err := call(raw, bsonDoc(elems...))
	if err != nil {
		return err
	}
	return mongoCmdErr(resp)
}
