package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateKeypair creates a new Ed25519 keypair.
func GenerateKeypair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ed25519 key: %w", err)
	}
	return pub, priv, nil
}

// Sign signs a message with an Ed25519 private key and returns a base64-encoded signature.
func Sign(message []byte, privateKey ed25519.PrivateKey) string {
	sig := ed25519.Sign(privateKey, message)
	return base64.StdEncoding.EncodeToString(sig)
}

// Verify checks an Ed25519 signature (base64-encoded) against a message and public key.
func Verify(message []byte, signature string, publicKey ed25519.PublicKey) bool {
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	return ed25519.Verify(publicKey, message, sig)
}

// Fingerprint returns the SHA256 hex fingerprint of a public key.
func Fingerprint(publicKey ed25519.PublicKey) string {
	hash := sha256.Sum256(publicKey)
	return hex.EncodeToString(hash[:])
}

// EncodePublicKey returns a base64-encoded public key.
func EncodePublicKey(key ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(key)
}

// DecodePublicKey decodes a base64-encoded public key.
func DecodePublicKey(encoded string) (ed25519.PublicKey, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	if len(data) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: %d", len(data))
	}
	return ed25519.PublicKey(data), nil
}

// EncodePrivateKey returns a base64-encoded private key.
func EncodePrivateKey(key ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(key)
}

// DecodePrivateKey decodes a base64-encoded private key.
func DecodePrivateKey(encoded string) (ed25519.PrivateKey, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	if len(data) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: %d", len(data))
	}
	return ed25519.PrivateKey(data), nil
}

// LoadOrGenerateKeypair loads a keypair from disk, or generates and saves one if it doesn't exist.
func LoadOrGenerateKeypair(dir, name string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pubPath := filepath.Join(dir, name+".pub")
	privPath := filepath.Join(dir, name+".key")

	// Try loading existing
	pubData, pubErr := os.ReadFile(pubPath)
	privData, privErr := os.ReadFile(privPath)

	if pubErr == nil && privErr == nil {
		pub, err := DecodePublicKey(string(pubData))
		if err != nil {
			return nil, nil, fmt.Errorf("load public key: %w", err)
		}
		priv, err := DecodePrivateKey(string(privData))
		if err != nil {
			return nil, nil, fmt.Errorf("load private key: %w", err)
		}
		return pub, priv, nil
	}

	// Generate new
	pub, priv, err := GenerateKeypair()
	if err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, nil, fmt.Errorf("create key directory: %w", err)
	}
	if err := os.WriteFile(pubPath, []byte(EncodePublicKey(pub)), 0644); err != nil {
		return nil, nil, fmt.Errorf("write public key: %w", err)
	}
	if err := os.WriteFile(privPath, []byte(EncodePrivateKey(priv)), 0600); err != nil {
		return nil, nil, fmt.Errorf("write private key: %w", err)
	}

	return pub, priv, nil
}

// LoadPublicKey loads a base64-encoded public key from a file.
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DecodePublicKey(string(data))
}

// LoadRegisteredClients loads all registered client public keys from a directory.
func LoadRegisteredClients(dir string) ([]ed25519.PublicKey, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var keys []ed25519.PublicKey
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".pub" {
			continue
		}
		key, err := LoadPublicKey(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// RegisterClient saves a client public key to the clients directory.
func RegisterClient(clientsDir string, publicKey ed25519.PublicKey) error {
	if err := os.MkdirAll(clientsDir, 0755); err != nil {
		return err
	}
	fp := Fingerprint(publicKey)
	path := filepath.Join(clientsDir, fp+".pub")
	return os.WriteFile(path, []byte(EncodePublicKey(publicKey)), 0644)
}

// IsRegisteredClient checks if a public key is in the list of registered clients.
func IsRegisteredClient(clients []ed25519.PublicKey, key ed25519.PublicKey) bool {
	for _, c := range clients {
		if c.Equal(key) {
			return true
		}
	}
	return false
}
