package crypto

import (
	"path/filepath"
	"testing"
)

func TestGenerateAndSignVerify(t *testing.T) {
	pub, priv, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}

	message := []byte("hello world")
	sig := Sign(message, priv)

	if !Verify(message, sig, pub) {
		t.Error("signature should be valid")
	}
	if Verify([]byte("tampered"), sig, pub) {
		t.Error("signature should be invalid for different message")
	}
}

func TestVerifyBadSignature(t *testing.T) {
	pub, _, _ := GenerateKeypair()
	if Verify([]byte("msg"), "not-valid-base64!!!", pub) {
		t.Error("should reject invalid base64")
	}
	if Verify([]byte("msg"), "dGVzdA==", pub) {
		t.Error("should reject wrong signature")
	}
}

func TestEncodeDecodePublicKey(t *testing.T) {
	pub, _, _ := GenerateKeypair()
	encoded := EncodePublicKey(pub)
	decoded, err := DecodePublicKey(encoded)
	if err != nil {
		t.Fatalf("DecodePublicKey: %v", err)
	}
	if !pub.Equal(decoded) {
		t.Error("round-trip should preserve key")
	}
}

func TestEncodeDecodePrivateKey(t *testing.T) {
	_, priv, _ := GenerateKeypair()
	encoded := EncodePrivateKey(priv)
	decoded, err := DecodePrivateKey(encoded)
	if err != nil {
		t.Fatalf("DecodePrivateKey: %v", err)
	}
	if !priv.Equal(decoded) {
		t.Error("round-trip should preserve key")
	}
}

func TestFingerprint(t *testing.T) {
	pub, _, _ := GenerateKeypair()
	fp := Fingerprint(pub)
	if len(fp) != 64 { // SHA256 hex = 64 chars
		t.Errorf("expected 64 char fingerprint, got %d", len(fp))
	}
}

func TestLoadOrGenerateKeypair(t *testing.T) {
	dir := t.TempDir()

	// First call: generates
	pub1, priv1, err := LoadOrGenerateKeypair(dir, "test")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Second call: loads
	pub2, priv2, err := LoadOrGenerateKeypair(dir, "test")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if !pub1.Equal(pub2) || !priv1.Equal(priv2) {
		t.Error("should load same keypair on second call")
	}
}

func TestRegisterAndLoadClients(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "clients")

	pub1, _, _ := GenerateKeypair()
	pub2, _, _ := GenerateKeypair()

	RegisterClient(dir, pub1)
	RegisterClient(dir, pub2)

	clients, err := LoadRegisteredClients(dir)
	if err != nil {
		t.Fatalf("LoadRegisteredClients: %v", err)
	}
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}

	if !IsRegisteredClient(clients, pub1) {
		t.Error("pub1 should be registered")
	}
	if !IsRegisteredClient(clients, pub2) {
		t.Error("pub2 should be registered")
	}

	pub3, _, _ := GenerateKeypair()
	if IsRegisteredClient(clients, pub3) {
		t.Error("pub3 should not be registered")
	}
}
