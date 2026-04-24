package config

import (
	"strings"
	"testing"
)

func TestDPAPIRoundTrip(t *testing.T) {
	plain := []byte("my-plex-token-12345")
	enc, err := dpapiEncrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if len(enc) == 0 {
		t.Fatal("ciphertext is empty")
	}
	if strings.Contains(string(enc), string(plain)) {
		t.Fatal("ciphertext contains plaintext — something is wrong")
	}
	dec, err := dpapiDecrypt(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(dec) != string(plain) {
		t.Fatalf("round-trip mismatch: got %q want %q", dec, plain)
	}
}

func TestDPAPIRejectsGarbage(t *testing.T) {
	_, err := dpapiDecrypt([]byte("this is not a DPAPI blob"))
	if err == nil {
		t.Fatal("expected error on garbage input, got nil")
	}
}
