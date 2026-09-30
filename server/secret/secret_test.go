package secret

import (
	"testing"

	"fluxguard/common/crypto"
)

func TestPasswordIsNotDecryptable(t *testing.T) {
	hashed, err := HashPassword("fluxguard")
	if err != nil {
		t.Fatal(err)
	}
	if !MatchPassword(hashed, "fluxguard") {
		t.Fatal("bcrypt should match")
	}
	stored, err := crypto.Encrypt(crypto.DefaultKey, "fluxguard")
	if err != nil {
		t.Fatal(err)
	}
	if MatchPassword(stored, "fluxguard") {
		t.Fatal("encrypted password must not be accepted")
	}
	if MatchPassword("fluxguard", "fluxguard") {
		t.Fatal("plaintext password must not be accepted")
	}
}
