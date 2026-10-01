package kafka

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash"
	"net"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

func saslScram(raw net.Conn, mechanism, user, secret string) error {
	body, err := roundTrip(raw, 17, 1, 2, kafkaString(mechanism))
	if err != nil || len(body) < 2 || binary.BigEndian.Uint16(body[:2]) != 0 {
		return fmt.Errorf("Kafka 认证失败")
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("Kafka 认证失败")
	}
	clientFirst := "n=" + scramUser(user) + ",r=" + base64.RawStdEncoding.EncodeToString(nonce)
	serverFirst, err := saslToken(raw, 3, "n,,"+clientFirst)
	if err != nil {
		return err
	}
	clientFinal, expect, err := scramProof(clientFirst, serverFirst, secret, mechanism)
	if err != nil {
		return fmt.Errorf("Kafka 认证失败")
	}
	serverFinal, err := saslToken(raw, 4, clientFinal)
	if err != nil {
		return err
	}
	if !scramServerOK(serverFinal, expect) {
		return fmt.Errorf("Kafka 认证失败")
	}
	return nil
}

func saslToken(raw net.Conn, corr int32, token string) (string, error) {
	body, err := roundTrip(raw, 36, 0, corr, kafkaBytes([]byte(token)))
	if err != nil {
		return "", fmt.Errorf("Kafka 认证失败")
	}
	r := reader{b: body}
	code, err := r.i16()
	if err != nil || code != 0 {
		return "", fmt.Errorf("Kafka 认证失败")
	}
	if _, _, err := r.nullable(); err != nil {
		return "", fmt.Errorf("Kafka 认证失败")
	}
	n, err := r.i32()
	if err != nil || n < 0 {
		return "", fmt.Errorf("Kafka 认证失败")
	}
	if n == 0 {
		return "", nil
	}
	b, err := r.take(int(n))
	if err != nil {
		return "", fmt.Errorf("Kafka 认证失败")
	}
	return string(b), nil
}

// scramProof 按 SCRAM 算出客户端最终消息，以及期望的服务端签名。
func scramProof(clientFirstBare, serverFirst, secret, mechanism string) (clientFinal, serverSig string, err error) {
	fields := scramFields(serverFirst)
	full := fields["r"]
	saltText := fields["s"]
	iterText := fields["i"]
	if full == "" || !strings.HasPrefix(full, scramNonce(clientFirstBare)) || saltText == "" || iterText == "" {
		return "", "", fmt.Errorf("bad server-first")
	}
	iter, err := strconv.Atoi(iterText)
	if err != nil || iter < 1 || iter > 1000000 {
		return "", "", fmt.Errorf("bad iterations")
	}
	salt, err := base64.StdEncoding.DecodeString(saltText)
	if err != nil {
		return "", "", err
	}
	h := scramHash(mechanism)
	if h == nil {
		return "", "", fmt.Errorf("bad mechanism")
	}
	return scramProofHash(clientFirstBare, full, serverFirst, salt, iter, secret, h)
}

func scramProofHash(clientFirstBare, full, serverFirst string, salt []byte, iter int, secret string, h func() hash.Hash) (clientFinal, serverSig string, err error) {
	salted := pbkdf2.Key([]byte(secret), salt, iter, h().Size(), h)
	clientKey := scramHMAC(h, salted, []byte("Client Key"))
	stored := h()
	stored.Write(clientKey)
	storedKey := stored.Sum(nil)
	without := "c=biws,r=" + full
	auth := clientFirstBare + "," + serverFirst + "," + without
	proof := xorBytes(clientKey, scramHMAC(h, storedKey, []byte(auth)))
	serverKey := scramHMAC(h, salted, []byte("Server Key"))
	sig := scramHMAC(h, serverKey, []byte(auth))
	return without + ",p=" + base64.StdEncoding.EncodeToString(proof), base64.StdEncoding.EncodeToString(sig), nil
}

func scramServerOK(serverFinal, expect string) bool {
	fields := scramFields(serverFinal)
	if fields["e"] != "" || fields["v"] == "" {
		return false
	}
	return fields["v"] == expect
}

func scramHash(mechanism string) func() hash.Hash {
	switch mechanism {
	case "SCRAM-SHA-256":
		return sha256.New
	case "SCRAM-SHA-512":
		return sha512.New
	default:
		return nil
	}
}

func scramUser(user string) string {
	user = strings.ReplaceAll(user, "=", "=3D")
	return strings.ReplaceAll(user, ",", "=2C")
}

func scramNonce(clientFirstBare string) string {
	for _, part := range strings.Split(clientFirstBare, ",") {
		if strings.HasPrefix(part, "r=") {
			return strings.TrimPrefix(part, "r=")
		}
	}
	return ""
}

func scramFields(msg string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(msg, ",") {
		key, value, ok := strings.Cut(part, "=")
		if ok && key != "" {
			out[key] = value
		}
	}
	return out
}

func scramHMAC(h func() hash.Hash, key, msg []byte) []byte {
	mac := hmac.New(h, key)
	mac.Write(msg)
	return mac.Sum(nil)
}

func xorBytes(a, b []byte) []byte {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = a[i] ^ b[i]
	}
	return out
}
