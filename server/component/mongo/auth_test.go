package mongo

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"fluxguard/server/component/conn"
)

func TestPlainPayload(t *testing.T) {
	got, err := plainPayload("user", "pencil")
	if err != nil {
		t.Fatal(err)
	}
	if base64.StdEncoding.EncodeToString(got) != "AHVzZXIAcGVuY2ls" {
		t.Fatal(base64.StdEncoding.EncodeToString(got))
	}
}

func TestAWSRegion(t *testing.T) {
	cases := map[string]string{
		"sts.amazonaws.com":           "us-east-1",
		"sts.us-west-2.amazonaws.com": "us-west-2",
		"example.com":                 "com",
		"localhost":                   "us-east-1",
	}
	for host, want := range cases {
		got, err := awsRegion(host)
		if err != nil || got != want {
			t.Fatalf("%s -> %s %v", host, got, err)
		}
	}
	if _, err := awsRegion("sts..com"); err == nil {
		t.Fatal("空标签应拒绝")
	}
}

func TestAWSHeaderUsesNonce(t *testing.T) {
	nonce := bytes32()
	when := time.Date(2019, 11, 7, 0, 26, 7, 0, time.UTC)
	header, date, err := awsAuthorization("AKIAEXAMPLE", "secret", "", "sts.amazonaws.com", nonce, when)
	if err != nil || date != "20191107T002607Z" {
		t.Fatal(header, date, err)
	}
	if !strings.Contains(header, "Credential=AKIAEXAMPLE/20191107/us-east-1/sts/aws4_request") {
		t.Fatal(header)
	}
	if !strings.Contains(header, "x-mongodb-server-nonce") || strings.Contains(header, "x-amz-security-token") {
		t.Fatal(header)
	}
	withToken, _, err := awsAuthorization("AKIAEXAMPLE", "secret", "session", "sts.amazonaws.com", nonce, when)
	if err != nil || !strings.Contains(withToken, "x-amz-security-token") || withToken == header {
		t.Fatal(withToken)
	}
}

func TestCollectPlain(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	errCh := make(chan error, 1)
	go func() { errCh <- servePlain(ln) }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, status := collect(ctx, conn.Open{
		Address: ln.Addr().String(), OptionText: "standalone:plain",
		UserName: "user", Secret: "pencil",
	})
	if !status.Up || status.Version != "7.0.0" {
		t.Fatalf("%+v", status)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func TestCollectX509(t *testing.T) {
	bundle, pool := testCert(t)
	mongoTestRoots = pool
	defer func() { mongoTestRoots = nil }()
	pair, err := tls.X509KeyPair(bundle, bundle)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{pair},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	errCh := make(chan error, 1)
	go func() { errCh <- serveX509(ln) }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, status := collect(ctx, conn.Open{
		Address: ln.Addr().String(), OptionText: "tls:x509", Secret: string(bundle),
	})
	if !status.Up || status.Version != "7.0.0" {
		t.Fatalf("%+v", status)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func TestCollectAWS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	errCh := make(chan error, 1)
	go func() { errCh <- serveAWS(ln) }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, status := collect(ctx, conn.Open{
		Address: ln.Addr().String(), OptionText: "standalone:aws",
		UserName: "AKIAEXAMPLE", Secret: "secret\nsession",
	})
	if !status.Up || status.Version != "7.0.0" {
		t.Fatalf("%+v", status)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func servePlain(ln net.Listener) error {
	c, err := ln.Accept()
	if err != nil {
		return err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	doc, err := readCmd(c)
	if err != nil {
		return err
	}
	want, err := plainPayload("user", "pencil")
	if err != nil {
		return err
	}
	if text(dig(doc, "mechanism")) != "PLAIN" || text(dig(doc, "$db")) != "$external" || string(bytesOf(dig(doc, "payload"))) != string(want) {
		return fmt.Errorf("PLAIN 报文不对")
	}
	if err := writeCmd(c, bsonDoc(bsonInt32("conversationId", 1), bsonBool("done", true), bsonBinary("payload", nil), bsonDouble("ok", 1))); err != nil {
		return err
	}
	return replyStatus(c)
}

func serveX509(ln net.Listener) error {
	c, err := ln.Accept()
	if err != nil {
		return err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	doc, err := readCmd(c)
	if err != nil {
		return err
	}
	if text(dig(doc, "mechanism")) != "MONGODB-X509" || text(dig(doc, "$db")) != "$external" || text(dig(doc, "user")) != "" {
		return fmt.Errorf("X.509 报文不对")
	}
	if err := writeCmd(c, bsonDoc(bsonDouble("ok", 1))); err != nil {
		return err
	}
	return replyStatus(c)
}

func serveAWS(ln net.Listener) error {
	c, err := ln.Accept()
	if err != nil {
		return err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	doc, err := readCmd(c)
	if err != nil {
		return err
	}
	if text(dig(doc, "mechanism")) != "MONGODB-AWS" || text(dig(doc, "$db")) != "$external" {
		return fmt.Errorf("AWS 报文不对")
	}
	first, err := decodeDoc(bytesOf(dig(doc, "payload")))
	if err != nil {
		return err
	}
	nonce := bytesOf(dig(first, "r"))
	if len(nonce) != 32 || intOfMust(dig(first, "p")) != 110 {
		return fmt.Errorf("客户端随机数不对")
	}
	serverNonce := append(append([]byte{}, nonce...), bytes32()...)
	host := "sts.us-west-2.amazonaws.com"
	if err := writeCmd(c, bsonDoc(
		bsonInt32("conversationId", 3),
		bsonBool("done", false),
		bsonBinary("payload", bsonDoc(bsonBinary("s", serverNonce), bsonString("h", host))),
		bsonDouble("ok", 1),
	)); err != nil {
		return err
	}
	doc, err = readCmd(c)
	if err != nil {
		return err
	}
	second, err := decodeDoc(bytesOf(dig(doc, "payload")))
	if err != nil {
		return err
	}
	when, err := time.Parse("20060102T150405Z", text(dig(second, "d")))
	if err != nil {
		return err
	}
	header, _, err := awsAuthorization("AKIAEXAMPLE", "secret", "session", host, serverNonce, when)
	if err != nil {
		return err
	}
	if text(dig(second, "a")) != header || text(dig(second, "t")) != "session" {
		return fmt.Errorf("AWS 签名不对")
	}
	if err := writeCmd(c, bsonDoc(bsonInt32("conversationId", 3), bsonBool("done", true), bsonBinary("payload", nil), bsonDouble("ok", 1))); err != nil {
		return err
	}
	return replyStatus(c)
}

func replyStatus(c net.Conn) error {
	for i := 0; i < 4; i++ {
		doc, err := readCmd(c)
		if err != nil {
			return err
		}
		if i == 0 && num(dig(doc, "serverStatus")) != 1 {
			return fmt.Errorf("应收 serverStatus")
		}
		body := bsonDoc(bsonDouble("ok", 0), bsonString("errmsg", "skip"))
		if i == 0 {
			body = bsonDoc(bsonDouble("ok", 1), bsonString("version", "7.0.0"), bsonString("process", "mongod"))
		}
		if err := writeCmd(c, body); err != nil {
			return err
		}
	}
	return nil
}

func testCert(t *testing.T) ([]byte, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "mongo"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certPEM) {
		t.Fatal("证书池")
	}
	return append(certPEM, keyPEM...), pool
}

func bytes32() []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = byte(i + 1)
	}
	return out
}
