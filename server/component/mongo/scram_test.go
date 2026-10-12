package mongo

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"fluxguard/server/component/conn"
)

func TestScramSHA256Vector(t *testing.T) {
	client, err := newScram("SCRAM-SHA-256", "user", "pencil", "rOprNGfwEbeRWgbNEkqO")
	if err != nil {
		t.Fatal(err)
	}
	if got := client.first(); got != "n,,n=user,r=rOprNGfwEbeRWgbNEkqO" {
		t.Fatal(got)
	}
	final, err := client.final("r=rOprNGfwEbeRWgbNEkqO%hvYDpWUa2RaTCAfuxFIlj)hNlF$k0,s=W22ZaJ0SNY7soEsUEjb6gQ==,i=4096")
	if err != nil {
		t.Fatal(err)
	}
	want := "c=biws,r=rOprNGfwEbeRWgbNEkqO%hvYDpWUa2RaTCAfuxFIlj)hNlF$k0,p=dHzbZapWIk4jUhN+Ute9ytag9zjfMHgsqmmiz7AndVQ="
	if final != want {
		t.Fatal(final)
	}
	if err := client.verify("v=6rriTRBi23WpRR/wtup+mMhUZUn/dB5nLTJRsjl95G4="); err != nil {
		t.Fatal(err)
	}
}

func TestScramSHA1Vector(t *testing.T) {
	client, err := newScram("SCRAM-SHA-1", "user", "pencil", "fyko+d2lbbFgONRv9qkxdawL")
	if err != nil {
		t.Fatal(err)
	}
	if got := client.first(); got != "n,,n=user,r=fyko+d2lbbFgONRv9qkxdawL" {
		t.Fatal(got)
	}
	final, err := client.final("r=fyko+d2lbbFgONRv9qkxdawL3rfcNHYJY1ZVvWVs7j,s=QSXCR+Q6sek8bf92,i=4096")
	if err != nil {
		t.Fatal(err)
	}
	want := "c=biws,r=fyko+d2lbbFgONRv9qkxdawL3rfcNHYJY1ZVvWVs7j,p=v0X8v3Bz2T0CJGbJQyF0X+HI4Ts="
	if final != want {
		t.Fatal(final)
	}
	if err := client.verify("v=rmF9pqV8S7suAoZWja4dJRkFsKQ="); err != nil {
		t.Fatal(err)
	}
}

func TestMongoScramSHA1Password(t *testing.T) {
	user, pass, err := mongoScramSecret("SCRAM-SHA-1", "user", "pencil")
	if err != nil || user != "user" || pass != "1c33006ec1ffd90f9cadcbcc0e118200" {
		t.Fatalf("%s %s %v", user, pass, err)
	}
	user, pass, err = mongoScramSecret("SCRAM-SHA-256", "a,b", "pen\u00a0cil")
	if err != nil || user != "a,b" || pass != "pen cil" {
		t.Fatalf("%s %q %v", user, pass, err)
	}
	if _, _, err := mongoScramSecret("SCRAM-SHA-256", "user", "bad\u0000pass"); err == nil {
		t.Fatal("控制字符应拒绝")
	}
}

func TestAuthMode(t *testing.T) {
	plan, err := authMode(conn.Open{OptionText: "standalone:password", UserName: "app", Secret: "secret"})
	if err != nil || plan.mechanism != "SCRAM-SHA-256" || plan.database != "admin" {
		t.Fatalf("%+v %v", plan, err)
	}
	plan, err = authMode(conn.Open{OptionText: "standalone:scram1", UserName: "app", Secret: "secret", DatabaseName: "appdb"})
	if err != nil || plan.mechanism != "SCRAM-SHA-1" || plan.database != "appdb" {
		t.Fatalf("%+v %v", plan, err)
	}
	plan, err = authMode(conn.Open{OptionText: "tls:scram256", UserName: "app", Secret: "secret", DatabaseName: "admin"})
	if err != nil || !plan.tls || plan.mechanism != "SCRAM-SHA-256" {
		t.Fatalf("%+v %v", plan, err)
	}
	plan, err = authMode(conn.Open{OptionText: "standalone:none", UserName: "app", Secret: "secret"})
	if err != nil || plan.mechanism != "" {
		t.Fatalf("%+v %v", plan, err)
	}
	if _, err := authMode(conn.Open{OptionText: "standalone:scram256", UserName: "app"}); err == nil {
		t.Fatal("缺少口令应拒绝")
	}
}

func TestBinaryPayloadRoundtrip(t *testing.T) {
	raw := []byte("n,,n=user,r=abc")
	doc, err := decodeDoc(bsonDoc(bsonBinary("payload", raw), bsonInt32("conversationId", 7)))
	if err != nil {
		t.Fatal(err)
	}
	if string(bytesOf(dig(doc, "payload"))) != string(raw) || intOfMust(dig(doc, "conversationId")) != 7 {
		t.Fatalf("%v", doc)
	}
}

func TestCollectScram(t *testing.T) {
	for _, extra := range []bool{false, true} {
		t.Run(fmt.Sprintf("extra-%v", extra), func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			errCh := make(chan error, 1)
			go func() {
				errCh <- serveScram(ln, extra, false)
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, status := collect(ctx, conn.Open{
				Address: ln.Addr().String(), OptionText: "standalone:scram256",
				UserName: "user", Secret: "pencil", DatabaseName: "admin",
			})
			if !status.Up || status.Version != "7.0.0" || status.Error != "" {
				t.Fatalf("%+v", status)
			}
			if err := <-errCh; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCollectScramRejected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- serveScram(ln, false, true)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, status := collect(ctx, conn.Open{
		Address: ln.Addr().String(), OptionText: "standalone:scram1",
		UserName: "user", Secret: "pencil",
	})
	if status.Up || !strings.Contains(status.Error, "认证失败") {
		t.Fatalf("%+v", status)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func serveScram(ln net.Listener, extra, reject bool) error {
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
	if num(dig(doc, "saslStart")) != 1 || text(dig(doc, "$db")) == "" {
		return fmt.Errorf("saslStart 无效")
	}
	if reject {
		return writeCmd(c, bsonDoc(bsonDouble("ok", 0), bsonString("errmsg", "Authentication failed.")))
	}
	mechanism := text(dig(doc, "mechanism"))
	var h func() hash.Hash
	switch mechanism {
	case "SCRAM-SHA-256":
		h = sha256.New
	case "SCRAM-SHA-1":
		h = sha1.New
	default:
		return fmt.Errorf("机制 %s", mechanism)
	}
	payload := string(bytesOf(dig(doc, "payload")))
	bare := strings.TrimPrefix(payload, "n,,")
	attrs, err := scramAttrs(bare)
	if err != nil {
		return err
	}
	salt := []byte("sixteen-byte-key")
	serverFirst := fmt.Sprintf("r=%sSrv,s=%s,i=4096", attrs["r"], base64.StdEncoding.EncodeToString(salt))
	if err := writeCmd(c, bsonDoc(
		bsonInt32("conversationId", 7),
		bsonBool("done", false),
		bsonBinary("payload", []byte(serverFirst)),
		bsonDouble("ok", 1),
	)); err != nil {
		return err
	}
	doc, err = readCmd(c)
	if err != nil {
		return err
	}
	if num(dig(doc, "saslContinue")) != 1 || intOfMust(dig(doc, "conversationId")) != 7 {
		return fmt.Errorf("saslContinue 无效")
	}
	clientFinal := string(bytesOf(dig(doc, "payload")))
	idx := strings.LastIndex(clientFinal, ",p=")
	if idx < 0 {
		return fmt.Errorf("缺少证明")
	}
	password := "pencil"
	if mechanism == "SCRAM-SHA-1" {
		_, password, err = mongoScramSecret(mechanism, "user", "pencil")
		if err != nil {
			return err
		}
	}
	proof, sig, err := scramProof(h, password, salt, 4096, bare+","+serverFirst+","+clientFinal[:idx])
	if err != nil {
		return err
	}
	got, err := base64.StdEncoding.DecodeString(clientFinal[idx+3:])
	if err != nil || subtle.ConstantTimeCompare(got, proof) != 1 {
		return fmt.Errorf("证明不匹配")
	}
	done := !extra
	if err := writeCmd(c, bsonDoc(
		bsonInt32("conversationId", 7),
		bsonBool("done", done),
		bsonBinary("payload", []byte("v="+base64.StdEncoding.EncodeToString(sig))),
		bsonDouble("ok", 1),
	)); err != nil {
		return err
	}
	if extra {
		doc, err = readCmd(c)
		if err != nil {
			return err
		}
		if len(bytesOf(dig(doc, "payload"))) != 0 {
			return fmt.Errorf("应收空载荷")
		}
		if err := writeCmd(c, bsonDoc(
			bsonInt32("conversationId", 7),
			bsonBool("done", true),
			bsonBinary("payload", nil),
			bsonDouble("ok", 1),
		)); err != nil {
			return err
		}
	}
	for i := 0; i < 4; i++ {
		doc, err = readCmd(c)
		if err != nil {
			return err
		}
		if i == 0 {
			if num(dig(doc, "serverStatus")) != 1 {
				return fmt.Errorf("应收 serverStatus")
			}
			if err := writeCmd(c, bsonDoc(bsonDouble("ok", 1), bsonString("version", "7.0.0"), bsonString("process", "mongod"))); err != nil {
				return err
			}
			continue
		}
		if err := writeCmd(c, bsonDoc(bsonDouble("ok", 0), bsonString("errmsg", "skip"))); err != nil {
			return err
		}
	}
	return nil
}

func readCmd(c net.Conn) (map[string]any, error) {
	var sizeBuf [4]byte
	if _, err := io.ReadFull(c, sizeBuf[:]); err != nil {
		return nil, err
	}
	size := int(binary.LittleEndian.Uint32(sizeBuf[:]))
	if size < 21 || size > 8*1024*1024 {
		return nil, fmt.Errorf("报文无效")
	}
	rest := make([]byte, size-4)
	if _, err := io.ReadFull(c, rest); err != nil {
		return nil, err
	}
	if len(rest) < 17 || rest[16] != 0 {
		return nil, fmt.Errorf("报文无效")
	}
	return decodeDoc(rest[17:])
}

func writeCmd(c net.Conn, doc []byte) error {
	body := make([]byte, 1+len(doc))
	copy(body[1:], doc)
	size := 16 + 4 + len(body)
	buf := make([]byte, size)
	binary.LittleEndian.PutUint32(buf[0:], uint32(size))
	binary.LittleEndian.PutUint32(buf[4:], 7)
	binary.LittleEndian.PutUint32(buf[8:], 1)
	binary.LittleEndian.PutUint32(buf[12:], 2013)
	copy(buf[20:], body)
	_, err := c.Write(buf)
	return err
}

func intOfMust(v any) int64 {
	n, _ := intOf(v)
	return n
}
