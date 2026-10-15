package secret

import (
	"bytes"
	"encoding/base64"
	"testing"
)

var testKey = bytes.Repeat([]byte{7}, 32)

func TestSealOpenRoundTrip(t *testing.T) {
	sealed, err := Seal(testKey, []byte("sk-ant-test-key-1234"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(testKey, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "sk-ant-test-key-1234" {
		t.Fatalf("got %q", got)
	}
}

func TestOpenWithAnotherKeyFails(t *testing.T) {
	sealed, err := Seal(testKey, []byte("sk-ant-test-key-1234"))
	if err != nil {
		t.Fatal(err)
	}
	other := bytes.Repeat([]byte{9}, 32)
	if _, err := Open(other, sealed); err == nil {
		t.Fatal("opening with another key should fail")
	}
}

func TestOpenTamperedFails(t *testing.T) {
	sealed, err := Seal(testKey, []byte("sk-ant-test-key-1234"))
	if err != nil {
		t.Fatal(err)
	}
	sealed[len(sealed)-1] ^= 0x01
	if _, err := Open(testKey, sealed); err == nil {
		t.Fatal("opening altered data should fail")
	}
	if _, err := Open(testKey, []byte("curto")); err == nil {
		t.Fatal("opening data shorter than the nonce should fail")
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	a, err := Seal(testKey, []byte("mesmo texto"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Seal(testKey, []byte("mesmo texto"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same text should differ")
	}
}

func TestParseKeyRejectsWrongLength(t *testing.T) {
	ok := base64.StdEncoding.EncodeToString(testKey)
	if key, err := ParseKey(ok); err != nil || len(key) != 32 {
		t.Fatalf("valid 32-byte key rejected: %v", err)
	}
	for _, raw := range []string{
		"",
		"não é base64!",
		base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 16)),
		base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 33)),
	} {
		if _, err := ParseKey(raw); err == nil {
			t.Errorf("ParseKey(%q) should fail", raw)
		}
	}
}
