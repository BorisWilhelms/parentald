package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"

	"github.com/BorisWilhelms/parentald/internal/version"
)

// CheckAndUpdate checks the server for a newer version and replaces the
// current binary if one is available. Returns true if an update was applied
// (caller should exit so systemd can restart with the new binary).
func CheckAndUpdate(serverURL string) (bool, error) {
	remoteVersion, err := fetchVersion(serverURL)
	if err != nil {
		return false, fmt.Errorf("fetch version: %w", err)
	}

	if remoteVersion == version.Version || remoteVersion == "" {
		return false, nil
	}

	binary, err := fetchBinary(serverURL)
	if err != nil {
		return false, fmt.Errorf("fetch binary: %w", err)
	}
	defer os.Remove(binary)

	execPath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("resolve executable path: %w", err)
	}

	if err := os.Rename(binary, execPath); err != nil {
		return false, fmt.Errorf("replace binary: %w", err)
	}

	return true, nil
}

func fetchVersion(serverURL string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/version", serverURL))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func fetchBinary(serverURL string) (string, error) {
	url := fmt.Sprintf("%s/api/daemon/%s/%s", serverURL, runtime.GOOS, runtime.GOARCH)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "parentald-update-*")
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}

	if err := tmp.Chmod(0755); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}

	tmp.Close()
	return tmp.Name(), nil
}
