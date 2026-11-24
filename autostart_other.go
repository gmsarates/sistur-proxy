//go:build !linux

package main

import "errors"

// TODO: Windows (atalho em shell:startup / chave Run) e macOS (LaunchAgent).
func setAutostart(enabled bool) error {
	if !enabled {
		return nil
	}
	return errors.New("iniciar com o sistema ainda não está disponível neste sistema operacional")
}
