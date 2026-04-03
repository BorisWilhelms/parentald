package activity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseExecField(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/usr/bin/firefox %u", "firefox"},
		{"firefox", "firefox"},
		{"firefox %F", "firefox"},
		{"/usr/lib/libreoffice/program/soffice --writer %U", "soffice"},
		{"env BAMF_DESKTOP_FILE_HINT=/usr/share/foo.desktop /usr/bin/foo", "foo"},
		{"env GDK_BACKEND=x11 /usr/bin/steam %U", "steam"},
		{"/usr/bin/flatpak run --branch=stable --arch=x86_64 com.valvesoftware.Steam", "com.valvesoftware.Steam"},
		{"/usr/bin/flatpak run com.example.App %U", "com.example.App"},
		{"", ""},
	}

	for _, tt := range tests {
		got := parseExecField(tt.input)
		if got != tt.want {
			t.Errorf("parseExecField(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFirstCategory(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Network;WebBrowser;", "Network"},
		{"Game;", "Game"},
		{"AudioVideo;Audio;", "AudioVideo"},
		{"", ""},
		{";;;", ""},
	}

	for _, tt := range tests {
		got := firstCategory(tt.input)
		if got != tt.want {
			t.Errorf("firstCategory(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDesktopLookup(t *testing.T) {
	dir := t.TempDir()

	// Write a test .desktop file
	content := `[Desktop Entry]
Name=Test Browser
Exec=/usr/bin/test-browser %u
Categories=Network;WebBrowser;
Type=Application
`
	os.WriteFile(filepath.Join(dir, "test-browser.desktop"), []byte(content), 0644)

	// Write one without categories
	content2 := `[Desktop Entry]
Name=My Tool
Exec=mytool --flag
Type=Application
`
	os.WriteFile(filepath.Join(dir, "mytool.desktop"), []byte(content2), 0644)

	dl := &DesktopLookup{apps: make(map[string]AppInfo)}
	dl.scanDir(dir)

	info, ok := dl.Lookup("test-browser")
	if !ok {
		t.Fatal("expected to find test-browser")
	}
	if info.Name != "Test Browser" {
		t.Errorf("Name = %q, want %q", info.Name, "Test Browser")
	}
	if info.Category == nil || *info.Category != "Network" {
		t.Errorf("Category = %v, want Network", info.Category)
	}

	info2, ok := dl.Lookup("mytool")
	if !ok {
		t.Fatal("expected to find mytool")
	}
	if info2.Name != "My Tool" {
		t.Errorf("Name = %q, want %q", info2.Name, "My Tool")
	}
	if info2.Category != nil {
		t.Errorf("Category = %v, want nil", info2.Category)
	}

	_, ok = dl.Lookup("nonexistent")
	if ok {
		t.Error("should not find nonexistent")
	}
}
