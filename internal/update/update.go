package update

import (
	"crypto/ed25519"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BorisWilhelms/parentald/internal/crypto"
	"github.com/BorisWilhelms/parentald/internal/version"
)

// CheckAndUpdate checks if remoteVersion differs from the current version
// and replaces the binary if so. The binary signature is verified with the
// server's public key. Returns true if an update was applied.
func CheckAndUpdate(serverURL, remoteVersion string, serverPub ed25519.PublicKey) (bool, error) {
	if remoteVersion == version.Version || remoteVersion == "" {
		return false, nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("resolve executable path: %w", err)
	}

	targetDir := filepath.Dir(execPath)
	binary, binaryData, err := fetchBinary(serverURL, targetDir)
	if err != nil {
		return false, fmt.Errorf("fetch binary: %w", err)
	}
	defer os.Remove(binary)

	// Verify binary signature if server public key is available
	if serverPub != nil && binaryData.signature != "" {
		if !crypto.Verify(binaryData.content, binaryData.signature, serverPub) {
			return false, fmt.Errorf("binary signature verification failed")
		}
	}

	if err := os.Rename(binary, execPath); err != nil {
		return false, fmt.Errorf("replace binary: %w", err)
	}

	return true, nil
}

type binaryResult struct {
	content   []byte
	signature string
}

func fetchBinary(serverURL string, targetDir string) (string, binaryResult, error) {
	url := fmt.Sprintf("%s/api/daemon/%s/%s", serverURL, runtime.GOOS, runtime.GOARCH)
	resp, err := http.Get(url)
	if err != nil {
		return "", binaryResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", binaryResult{}, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", binaryResult{}, err
	}

	tmp, err := os.CreateTemp(targetDir, "parentald-update-*")
	if err != nil {
		return "", binaryResult{}, err
	}

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", binaryResult{}, err
	}

	if err := tmp.Chmod(0755); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", binaryResult{}, err
	}

	tmp.Close()
	return tmp.Name(), binaryResult{
		content:   content,
		signature: resp.Header.Get("X-Signature"),
	}, nil
}
