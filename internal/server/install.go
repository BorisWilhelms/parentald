package server

import (
	"crypto/ed25519"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BorisWilhelms/parentald/internal/crypto"
)

type installHandlers struct {
	binDir     string
	serverPub  ed25519.PublicKey
	serverPriv ed25519.PrivateKey
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
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		http.Error(w, "binary not found for this platform", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Sign the binary
	if h.serverPriv != nil {
		sig := crypto.Sign(data, h.serverPriv)
		w.Header().Set("X-Signature", sig)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=parentald-%s-%s", goos, goarch))
	w.Write(data)
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
	serverPubKey := crypto.EncodePublicKey(h.serverPub)

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, installScript, serverURL, serverPubKey)
}

const installScript = `#!/bin/bash
set -euo pipefail

SERVER_URL="%s"
SERVER_PUBKEY="%s"
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

# Save server public key
echo "$SERVER_PUBKEY" > "${CONFIG_DIR}/server.pub"
echo "  Saved server public key"

# Generate client keypair if not exists
if [ ! -f "${CONFIG_DIR}/client.key" ]; then
    # Use openssl to generate Ed25519 keypair, extract raw keys, base64 encode
    TMPKEY=$(mktemp)
    openssl genpkey -algorithm ed25519 -out "$TMPKEY" 2>/dev/null
    # Extract raw private key (last 32 bytes of DER-encoded seed + public key = 64 bytes)
    CLIENT_PRIV=$(openssl pkey -in "$TMPKEY" -outform DER 2>/dev/null | tail -c 64 | base64 -w0)
    CLIENT_PUB=$(openssl pkey -in "$TMPKEY" -pubout -outform DER 2>/dev/null | tail -c 32 | base64 -w0)
    rm -f "$TMPKEY"
    echo "$CLIENT_PRIV" > "${CONFIG_DIR}/client.key"
    chmod 600 "${CONFIG_DIR}/client.key"
    echo "$CLIENT_PUB" > "${CONFIG_DIR}/client.pub"
    echo "  Generated client keypair"

    # Register with server
    HTTP_CODE=$(curl -sS -o /dev/null -w "%%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "{\"publicKey\": \"${CLIENT_PUB}\"}" \
        "${SERVER_URL}/api/register")
    if [ "$HTTP_CODE" = "201" ] || [ "$HTTP_CODE" = "200" ]; then
        echo "  Registered client with server"
    else
        echo "Warning: failed to register client (HTTP $HTTP_CODE)"
    fi
else
    echo "  Client keypair already exists, skipping"
fi

# Write environment file (always overwrite to ensure correct values)
cat > "${CONFIG_DIR}/daemon.env" <<ENVEOF
SERVER_URL=${SERVER_URL}
ENVEOF
echo "  Created ${CONFIG_DIR}/daemon.env"

# Install systemd service
cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<SVCEOF
[Unit]
Description=parentald enforcement daemon
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/parentald --server=\${SERVER_URL}
Restart=always
RestartSec=5
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
