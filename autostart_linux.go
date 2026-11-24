//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func autostartPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", "evoproxy.desktop"), nil
}

// setAutostart cria/remove o atalho em ~/.config/autostart.
func setAutostart(enabled bool) error {
	path, err := autostartPath()
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	desktop := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Comment=Túnel da Evolution API pelo IP desta máquina
Exec="%s" --minimized
Icon=evoproxy
Terminal=false
X-GNOME-Autostart-enabled=true
`, appName, exe)
	return os.WriteFile(path, []byte(desktop), 0o644)
}
