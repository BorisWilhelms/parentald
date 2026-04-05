package activity

import (
	"os"
	"testing"
)

func TestIsOwnedBy(t *testing.T) {
	pid := os.Getpid()
	uid := os.Getuid()

	if !isOwnedBy(pid, uid) {
		t.Errorf("current process (PID %d) should be owned by UID %d", pid, uid)
	}
	if isOwnedBy(pid, uid+99999) {
		t.Error("should not match wrong UID")
	}
}

func TestIsOwnedBy_InvalidPID(t *testing.T) {
	if isOwnedBy(-1, 0) {
		t.Error("invalid PID should return false")
	}
}

func TestParseAppIDFromCgroupData(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"flatpak app",
			"0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-flatpak-com.valvesoftware.Steam-12345.scope",
			"com.valvesoftware.Steam",
		},
		{
			"gnome native app",
			"0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-gnome-steam-12345.scope",
			"steam",
		},
		{
			"gnome app with dots",
			"0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-gnome-org.mozilla.firefox-54321.scope",
			"org.mozilla.firefox",
		},
		{
			"dbus activated app",
			"0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-dbus-org.freedesktop.Foo-99999.scope",
			"org.freedesktop.Foo",
		},
		{
			"app with escaped hyphens",
			"0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-gnome-gnome\\x2dterminal\\x2dserver-12345.scope",
			"gnome-terminal-server",
		},
		{
			"flatpak with escaped hyphens",
			"0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-flatpak-io.github.some\\x2dapp-12345.scope",
			"io.github.some-app",
		},
		{
			"no app scope",
			"0::/user.slice/user-1000.slice/user@1000.service/init.scope",
			"",
		},
		{
			"system process",
			"0::/system.slice/sshd.service",
			"",
		},
		{
			"empty",
			"",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAppIDFromCgroupData(tt.input)
			if got != tt.want {
				t.Errorf("parseAppIDFromCgroupData(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
