package crypto

import "testing"

func TestRoundTrip(t *testing.T) {
	stored, err := Encrypt(DefaultKey, "fluxguard")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(stored) {
		t.Fatal(stored)
	}
	plain, err := Decrypt(DefaultKey, stored)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "fluxguard" {
		t.Fatalf("plain=%s", plain)
	}
	again, err := Decrypt("", "fluxguard")
	if err != nil || again != "fluxguard" {
		t.Fatalf("plain passthrough %q %v", again, err)
	}
}
