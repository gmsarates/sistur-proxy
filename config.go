package main

import (
	"errors"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
)

// Config fica salva nas preferências internas do app
// (Linux: ~/.config/fyne/app.evoproxy/preferences.json). Não precisa editar à mão.
type Config struct {
	RelayURL    string
	User        string
	Password    string
	AutoConnect bool
	AutoStart   bool
}

func (c Config) Complete() bool {
	return c.RelayURL != "" && c.User != "" && c.Password != ""
}

func loadConfig(p fyne.Preferences) Config {
	return Config{
		RelayURL:    p.String("relayURL"),
		User:        p.String("user"),
		Password:    p.String("password"),
		AutoConnect: p.BoolWithFallback("autoConnect", true),
		AutoStart:   p.Bool("autoStart"),
	}
}

func saveConfig(p fyne.Preferences, c Config) {
	p.SetString("relayURL", c.RelayURL)
	p.SetString("user", c.User)
	p.SetString("password", c.Password)
	p.SetBool("autoConnect", c.AutoConnect)
	p.SetBool("autoStart", c.AutoStart)
}

// normalizeRelayURL aceita "meu-relay.up.railway.app" e completa com https://.
func normalizeRelayURL(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "/")
	if s != "" && !strings.Contains(s, "://") {
		s = "https://" + s
	}
	return s
}

func validateRelayURL(s string) error {
	s = normalizeRelayURL(s)
	if s == "" {
		return errors.New("informe o endereço do relay")
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return errors.New("endereço inválido")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return errors.New("use https://")
	}
	return nil
}

func validateNotEmpty(field string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("informe " + field)
		}
		return nil
	}
}
