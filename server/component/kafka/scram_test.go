package kafka

import (
	"crypto/sha1"
	"encoding/base64"
	"testing"

	"fluxguard/server/component/conn"
)

func TestScramRFC5802(t *testing.T) {
	clientFirst := "n=user,r=fyko+d2lbbFgONRv9qkxdawL"
	full := "fyko+d2lbbFgONRv9qkxdawL3rfcNHYJY1ZVvWVs7j"
	serverFirst := "r=" + full + ",s=QSXCR+Q6sek8bf92,i=4096"
	salt, err := base64.StdEncoding.DecodeString("QSXCR+Q6sek8bf92")
	if err != nil {
		t.Fatal(err)
	}
	final, sig, err := scramProofHash(clientFirst, full, serverFirst, salt, 4096, "pencil", sha1.New)
	if err != nil {
		t.Fatal(err)
	}
	want := "c=biws,r=fyko+d2lbbFgONRv9qkxdawL3rfcNHYJY1ZVvWVs7j,p=v0X8v3Bz2T0CJGbJQyF0X+HI4Ts="
	if final != want || sig != "rmF9pqV8S7suAoZWja4dJRkFsKQ=" || !scramServerOK("v="+sig, sig) {
		t.Fatalf("%s %s", final, sig)
	}
	got, _, err := scramProof(clientFirst, serverFirst, "pencil", "SCRAM-SHA-256")
	if err != nil || got == "" || got == want {
		t.Fatalf("%s %v", got, err)
	}
}

func TestKafkaLink(t *testing.T) {
	opt := linkOf(conn.Open{OptionText: "tls:scram256", UserName: "app", Secret: "secret"})
	if !opt.tlsOn || opt.sasl != "SCRAM-SHA-256" || opt.user != "app" {
		t.Fatalf("%+v", opt)
	}
	plain := linkOf(conn.Open{OptionText: "cluster:plain", UserName: "app", Secret: "secret"})
	if plain.tlsOn || plain.sasl != "PLAIN" {
		t.Fatalf("%+v", plain)
	}
	none := linkOf(conn.Open{OptionText: "cluster:none", UserName: "app", Secret: "secret"})
	if none.sasl != "" {
		t.Fatalf("%+v", none)
	}
}
