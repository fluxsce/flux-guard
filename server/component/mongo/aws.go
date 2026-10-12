package mongo

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

func authenticateAWS(raw net.Conn, plan authPlan) error {
	accessKey := strings.TrimSpace(plan.user)
	secretKey, token := awsSecret(plan.secret)
	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("AWS 认证需要访问密钥编号和访问密钥")
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	resp, err := call(raw, bsonDoc(
		bsonInt32("saslStart", 1),
		bsonString("mechanism", "MONGODB-AWS"),
		bsonBinary("payload", bsonDoc(bsonBinary("r", nonce[:]), bsonInt32("p", 110))),
		bsonString("$db", plan.database),
	))
	if err != nil {
		return err
	}
	if err := mongoCmdErr(resp); err != nil {
		return err
	}
	conv, ok := intOf(dig(resp, "conversationId"))
	if !ok {
		return fmt.Errorf("MongoDB 认证应答无效")
	}
	server, err := decodeDoc(bytesOf(dig(resp, "payload")))
	if err != nil {
		return err
	}
	serverNonce := bytesOf(dig(server, "s"))
	host := text(dig(server, "h"))
	if len(serverNonce) != 64 || string(serverNonce[:32]) != string(nonce[:]) || !awsHostOK(host) {
		return fmt.Errorf("MongoDB 认证应答无效")
	}
	header, amzDate, err := awsAuthorization(accessKey, secretKey, token, host, serverNonce, time.Now().UTC())
	if err != nil {
		return err
	}
	elems := [][]byte{bsonString("a", header), bsonString("d", amzDate)}
	if token != "" {
		elems = append(elems, bsonString("t", token))
	}
	resp, err = saslContinue(raw, plan.database, conv, bsonDoc(elems...))
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

func awsSecret(secret string) (key, token string) {
	key, token, _ = strings.Cut(secret, "\n")
	return strings.TrimSpace(key), strings.TrimSpace(token)
}

func awsHostOK(host string) bool {
	if host == "" || len(host) > 255 || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") || strings.Contains(host, "..") {
		return false
	}
	return true
}

func awsRegion(host string) (string, error) {
	if !awsHostOK(host) {
		return "", fmt.Errorf("MongoDB 认证应答无效")
	}
	if host == "sts.amazonaws.com" || host == "aws.amazonaws.com" || !strings.Contains(host, ".") {
		return "us-east-1", nil
	}
	part := strings.Split(host, ".")[1]
	if part == "" {
		return "", fmt.Errorf("MongoDB 认证应答无效")
	}
	return part, nil
}

func awsAuthorization(accessKey, secretKey, token, host string, serverNonce []byte, now time.Time) (string, string, error) {
	region, err := awsRegion(host)
	if err != nil {
		return "", "", err
	}
	amzDate := now.Format("20060102T150405Z")
	day := now.Format("20060102")
	body := "Action=GetCallerIdentity&Version=2011-06-15"
	nonce := base64.StdEncoding.EncodeToString(serverNonce)
	names := []string{"content-length", "content-type", "host", "x-amz-date"}
	lines := []string{
		"content-length:43",
		"content-type:application/x-www-form-urlencoded",
		"host:" + host,
		"x-amz-date:" + amzDate,
	}
	if token != "" {
		names = append(names, "x-amz-security-token")
		lines = append(lines, "x-amz-security-token:"+token)
	}
	names = append(names, "x-mongodb-gs2-cb-flag", "x-mongodb-server-nonce")
	lines = append(lines, "x-mongodb-gs2-cb-flag:n", "x-mongodb-server-nonce:"+nonce)
	signed := strings.Join(names, ";")
	canonical := "POST\n/\n\n" + strings.Join(lines, "\n") + "\n\n" + signed + "\n" + sha256Hex(body)
	scope := day + "/" + region + "/sts/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex(canonical)
	signature := hex.EncodeToString(awsSign(secretKey, day, region, stringToSign))
	header := "AWS4-HMAC-SHA256 Credential=" + accessKey + "/" + scope + ", SignedHeaders=" + signed + ", Signature=" + signature
	return header, amzDate, nil
}

func awsSign(secret, day, region, payload string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), day)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "sts")
	kSigning := hmacSHA256(kService, "aws4_request")
	return hmacSHA256(kSigning, payload)
}

func hmacSHA256(key []byte, msg string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(msg))
	return mac.Sum(nil)
}

func sha256Hex(msg string) string {
	sum := sha256.Sum256([]byte(msg))
	return hex.EncodeToString(sum[:])
}
