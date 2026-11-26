package main

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
)

// Config fica salva nas preferências internas do app
// (Linux: ~/.config/fyne/app.evoproxy/preferences.json). Não precisa editar à mão.
// DefaultPort é a porta do túnel deste PC no relay. O relay usa os aparelhos na ordem
// das portas (TUNNEL_PORTS, padrão 1081..1090): TV box 1081, PC 1082, celular 1083...
const DefaultPort = 1082

type Config struct {
	RelayURL    string
	User        string
	Password    string
	Port        int
	AutoConnect bool
	AutoStart   bool
}

func (c Config) Complete() bool {
	return c.RelayURL != "" && c.User != "" && c.Password != "" && c.Port > 0
}

func loadConfig(p fyne.Preferences) Config {
	return Config{
		RelayURL:    p.String("relayURL"),
		User:        p.String("user"),
		Password:    p.String("password"),
		Port:        p.IntWithFallback("port", DefaultPort),
		AutoConnect: p.BoolWithFallback("autoConnect", true),
		AutoStart:   p.Bool("autoStart"),
	}
}

func saveConfig(p fyne.Preferences, c Config) {
	p.SetString("relayURL", c.RelayURL)
	p.SetString("user", c.User)
	p.SetString("password", c.Password)
	p.SetInt("port", c.Port)
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

func validatePort(s string) error {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > 65535 {
		return errors.New("porta entre 1 e 65535")
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
