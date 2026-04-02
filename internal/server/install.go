package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BorisWilhelms/parentald/internal/version"
)

type installHandlers struct {
	binDir string
}

func (h *installHandlers) serveVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, version.Version)
}

func (h *installHandlers) serveBinary(w http.ResponseWriter, r *http.Request) {
	goos := r.PathValue("os")
	goarch := r.PathValue("arch")

	// Sanitize to prevent path traversal
	if strings.Contains(goos, "/") || strings.Contains(goos, "..") ||
		strings.Contains(goarch, "/") || strings.Contains(goarch, "..") {
		http.Error(w, "invalid os/arch", http.StatusBadRequest)
		return
	}

	path := filepath.Join(h.binDir, fmt.Sprintf("parentald-%s-%s", goos, goarch))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, "binary not found for this platform", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=parentald-%s-%s", goos, goarch))
	http.ServeFile(w, r, path)
}

func (h *installHandlers) serveInstallScript(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	serverURL := fmt.Sprintf("%s://%s", scheme, r.Host)

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, installScript, serverURL)
}

const installScript = `#!/bin/bash
set -euo pipefail

SERVER_URL="%s"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/parentald"
SERVICE_NAME="parentald"

# Detect platform
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    armv7l)  ARCH="arm" ;;
esac

echo "parentald installer"
echo "  Server:   $SERVER_URL"
echo "  Platform: ${OS}/${ARCH}"
echo ""

# Download binary
echo "Downloading daemon binary..."
TMP=$(mktemp)
HTTP_CODE=$(curl -sS -o "$TMP" -w "%%{http_code}" "${SERVER_URL}/api/daemon/${OS}/${ARCH}")
if [ "$HTTP_CODE" != "200" ]; then
    echo "Error: failed to download binary (HTTP $HTTP_CODE)"
    rm -f "$TMP"
    exit 1
fi
chmod +x "$TMP"
mv "$TMP" "${INSTALL_DIR}/parentald"

# Fix SELinux context if SELinux is active
if command -v restorecon &> /dev/null; then
    restorecon -v "${INSTALL_DIR}/parentald"
fi

echo "  Installed to ${INSTALL_DIR}/parentald"

# Create config directory
mkdir -p "$CONFIG_DIR"

# Write environment file
if [ ! -f "${CONFIG_DIR}/daemon.env" ]; then
    cat > "${CONFIG_DIR}/daemon.env" <<ENVEOF
SERVER_URL=${SERVER_URL}
ENVEOF
    echo "  Created ${CONFIG_DIR}/daemon.env"
else
    echo "  ${CONFIG_DIR}/daemon.env already exists, skipping"
fi

# Install systemd service
cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<SVCEOF
[Unit]
Description=parentald enforcement daemon
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/parentald --server=\${SERVER_URL}
Restart=on-failure
EnvironmentFile=-${CONFIG_DIR}/daemon.env

[Install]
WantedBy=multi-user.target
SVCEOF
echo "  Installed systemd service"

# Configure PAM
PAM_LINE="auth required pam_listfile.so onerr=succeed item=user sense=deny file=${CONFIG_DIR}/deny-users"

configure_pam() {
    local pam_file="$1"
    if [ -f "$pam_file" ]; then
        if ! grep -q "parentald" "$pam_file"; then
            # Insert before the first auth line
            sed -i "0,/^auth/{s|^auth|${PAM_LINE}\nauth|}" "$pam_file"
            echo "  Configured PAM: $pam_file"
        else
            echo "  PAM already configured: $pam_file"
        fi
    fi
}

# Configure common display managers and login
configure_pam "/etc/pam.d/gdm-password"
configure_pam "/etc/pam.d/sddm"
configure_pam "/etc/pam.d/login"

# Enable and start service
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"
echo ""
echo "Done. parentald daemon is running."
echo "  Status: systemctl status parentald"
echo "  Logs:   journalctl -u parentald -f"
`
