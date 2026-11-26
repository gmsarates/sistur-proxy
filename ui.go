package main

import (
	"fmt"
	"image/color"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type state int

const (
	stNotConfigured state = iota
	stDisconnected
	stConnecting
	stConnected
	stReconnecting
	stError
)

type UI struct {
	app       fyne.App
	win       fyne.Window
	desk      desktop.App
	minimized bool
	noTray    bool // a área de notificação não está disponível neste desktop

	cfg    Config
	tunnel Tunnel
	state  state

	dot         *canvas.Circle
	statusLabel *widget.Label
	detailLabel *widget.Label
	relayLabel  *widget.Label
	portLabel   *widget.Label
	ipLabel     *widget.Label
	evoLabel    *widget.Label
	connectBtn  *widget.Button

	logLabel  *widget.Label
	logScroll *container.Scroll
	logBuf    []string

	connectedSince time.Time
	portBusy       bool // o relay recusou a porta: outro aparelho está usando
}

func newUI(a fyne.App, minimized bool) *UI {
	u := &UI{app: a, minimized: minimized}
	u.cfg = loadConfig(a.Preferences())
	if d, ok := a.(desktop.App); ok {
		u.desk = d
	}
	return u
}

func (u *UI) run() {
	u.app.SetIcon(resAppIcon)
	u.win = u.app.NewWindow(appName)
	u.win.SetIcon(resAppIcon)
	u.win.SetContent(u.build())
	u.win.Resize(fyne.NewSize(560, 520))

	if u.desk != nil {
		// Fechar a janela só esconde: o túnel continua rodando na bandeja.
		u.win.SetCloseIntercept(func() {
			if u.noTray {
				// Sem bandeja não haveria como reabrir a janela: fecha de verdade.
				u.tunnel.Stop()
				u.app.Quit()
				return
			}
			u.win.Hide()
		})
	}

	go u.consumeLogs()
	go u.tickUptime()

	if u.cfg.Complete() {
		u.setState(stDisconnected, "")
		if u.cfg.AutoConnect {
			u.connect()
		}
	} else {
		u.setState(stNotConfigured, "")
	}

	if !u.minimized || !u.cfg.Complete() || u.desk == nil {
		u.win.Show()
	}
	if !u.cfg.Complete() {
		u.showSettings(true)
	}
	u.app.Run()
}

func (u *UI) build() fyne.CanvasObject {
	u.dot = canvas.NewCircle(colGray)
	dotBox := container.NewGridWrap(fyne.NewSize(16, 16), u.dot)

	u.statusLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	u.detailLabel = widget.NewLabel("")
	u.detailLabel.Wrapping = fyne.TextWrapWord
	u.detailLabel.Importance = widget.LowImportance

	u.connectBtn = widget.NewButtonWithIcon("Conectar", theme.MediaPlayIcon(), u.toggleConnection)
	u.connectBtn.Importance = widget.HighImportance
	settingsBtn := widget.NewButtonWithIcon("Configurações", theme.SettingsIcon(), func() { u.showSettings(false) })

	header := container.NewBorder(nil, nil,
		container.NewCenter(dotBox),
		container.NewHBox(settingsBtn, u.connectBtn),
		container.NewVBox(u.statusLabel, u.detailLabel),
	)

	u.relayLabel = widget.NewLabel("")
	u.relayLabel.Truncation = fyne.TextTruncateEllipsis
	u.portLabel = widget.NewLabel("")
	u.ipLabel = widget.NewLabel("—")
	u.evoLabel = widget.NewLabel("")
	u.evoLabel.Wrapping = fyne.TextWrapWord

	info := widget.NewForm(
		widget.NewFormItem("Relay", u.relayLabel),
		widget.NewFormItem("Porta do túnel", u.portLabel),
		widget.NewFormItem("IP de saída", u.ipLabel),
		widget.NewFormItem("Na Evolution", u.evoLabel),
	)

	u.logLabel = widget.NewLabel("")
	u.logLabel.TextStyle = fyne.TextStyle{Monospace: true}
	u.logLabel.Wrapping = fyne.TextWrapBreak
	u.logScroll = container.NewVScroll(u.logLabel)

	logCard := widget.NewCard("", "Log", u.logScroll)

	top := container.NewVBox(
		container.NewPadded(header),
		widget.NewSeparator(),
		info,
	)
	u.refreshInfo()
	return container.NewPadded(container.NewBorder(top, nil, nil, nil, logCard))
}

func (u *UI) refreshInfo() {
	if u.cfg.RelayURL == "" {
		u.relayLabel.SetText("não configurado")
	} else {
		u.relayLabel.SetText(u.cfg.RelayURL)
	}
	u.portLabel.SetText(fmt.Sprintf("%d · o relay usa os aparelhos na ordem das portas (1081, 1082, 1083…)", u.cfg.Port))
	u.evoLabel.SetText("proxy socks5 · host <nome-do-relay>.railway.internal · porta 1080")
}

// ---------- conexão ----------

func (u *UI) toggleConnection() {
	if u.tunnel.Running() {
		u.disconnect()
	} else {
		u.connect()
	}
}

func (u *UI) connect() {
	u.portBusy = false
	if !u.cfg.Complete() {
		u.showSettings(false)
		return
	}
	u.setState(stConnecting, "")
	u.appendLog("", "Iniciando túnel…")
	err := u.tunnel.Start(u.cfg, func(err error) {
		fyne.Do(func() {
			msg := "o túnel parou"
			if err != nil {
				msg = err.Error()
			}
			u.appendLog("", "Túnel encerrado: "+msg)
			u.setState(stError, msg)
		})
	})
	if err != nil {
		u.appendLog("", "Não foi possível iniciar: "+err.Error())
		u.setState(stError, err.Error())
	}
}

func (u *UI) disconnect() {
	u.tunnel.Stop()
	u.appendLog("", "Desconectado por você")
	u.setState(stDisconnected, "")
}

func (u *UI) setState(s state, detail string) {
	u.state = s
	var col color.NRGBA
	var title string
	var tray fyne.Resource
	switch s {
	case stNotConfigured:
		col, title, tray = colGray, "Não configurado", resTrayGray
		if detail == "" {
			detail = "Abra Configurações e informe os dados do relay."
		}
	case stDisconnected:
		col, title, tray = colGray, "Desconectado", resTrayGray
		if detail == "" {
			detail = "A Evolution não tem saída enquanto o túnel estiver desligado."
		}
	case stConnecting:
		col, title, tray = colAmber, "Conectando…", resTrayAmber
	case stConnected:
		col, title, tray = colGreen, "Conectado", resTrayGreen
		if u.connectedSince.IsZero() {
			u.connectedSince = time.Now()
		}
	case stReconnecting:
		col, title, tray = colAmber, "Reconectando…", resTrayAmber
	case stError:
		col, title, tray = colRed, "Erro", resTrayRed
	}
	if s != stConnected {
		u.connectedSince = time.Time{}
	}
	if s == stConnected && detail == "" {
		detail = "Tudo que a Evolution envia está saindo por esta máquina."
	}
	u.dot.FillColor = col
	u.dot.Refresh()
	u.statusLabel.SetText(title)
	u.detailLabel.SetText(detail)

	running := u.tunnel.Running()
	if running {
		u.connectBtn.SetText("Desconectar")
		u.connectBtn.SetIcon(theme.MediaStopIcon())
		u.connectBtn.Importance = widget.MediumImportance
	} else {
		u.connectBtn.SetText("Conectar")
		u.connectBtn.SetIcon(theme.MediaPlayIcon())
		u.connectBtn.Importance = widget.HighImportance
	}
	u.connectBtn.Refresh()
	if s != stConnected {
		u.ipLabel.SetText("—")
	}

	if u.desk != nil {
		u.desk.SetSystemTrayIcon(tray)
		u.updateTrayMenu(title, running)
	}
}

func (u *UI) updateTrayMenu(status string, running bool) {
	toggleLabel := "Conectar"
	if running {
		toggleLabel = "Desconectar"
	}
	statusItem := fyne.NewMenuItem(status, nil)
	statusItem.Disabled = true
	quit := fyne.NewMenuItem("Sair", func() {
		u.tunnel.Stop()
		u.app.Quit()
	})
	quit.IsQuit = true
	u.desk.SetSystemTrayMenu(fyne.NewMenu(appName,
		statusItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Abrir janela", func() { u.win.Show(); u.win.RequestFocus() }),
		fyne.NewMenuItem(toggleLabel, u.toggleConnection),
		fyne.NewMenuItemSeparator(),
		quit,
	))
}

func (u *UI) tickUptime() {
	for range time.Tick(30 * time.Second) {
		fyne.Do(func() {
			if u.state == stConnected && !u.connectedSince.IsZero() {
				d := time.Since(u.connectedSince).Round(time.Minute)
				u.detailLabel.SetText("Conectado há " + humanDuration(d) + ". A Evolution está saindo por esta máquina.")
			}
		})
	}
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return "menos de 1 min"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h == 0 {
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%dh%02d", h, m)
}

// ---------- logs ----------

func (u *UI) consumeLogs() {
	for line := range logLines {
		clock, msg, ev := parseLogLine(line)
		fyne.Do(func() {
			if strings.Contains(msg, "systray error") {
				u.noTray = true
				u.win.Show()
				msg = "Área de notificação (bandeja) indisponível neste desktop: fechar a janela vai encerrar o túnel."
			}
			u.appendLog(clock, msg)
			if !u.tunnel.Running() {
				return
			}
			switch ev {
			case evConnecting:
				if u.state != stReconnecting {
					u.setState(stConnecting, "")
				}
			case evConnected:
				u.portBusy = false
				u.setState(stConnected, "")
				go u.fetchExitIP()
			case evReconnecting:
				if u.portBusy && strings.HasPrefix(msg, "Tentando de novo") {
					u.setState(stReconnecting, u.portBusyMessage()+" "+msg+".")
				} else {
					u.portBusy = false
					u.setState(stReconnecting, msg)
				}
			case evPortBusy:
				u.portBusy = true
				u.setState(stReconnecting, u.portBusyMessage())
			case evAuthFailed:
				u.tunnel.Stop()
				u.setState(stError, "Usuário ou senha do relay incorretos. Confira em Configurações.")
			}
		})
	}
}

func (u *UI) portBusyMessage() string {
	return fmt.Sprintf("A porta %d já está em uso por outro aparelho no relay. Escolha outra em Configurações.", u.cfg.Port)
}

func (u *UI) appendLog(clock, msg string) {
	if clock == "" {
		clock = time.Now().Format("15:04:05")
	}
	u.logBuf = append(u.logBuf, clock+"  "+msg)
	if len(u.logBuf) > 300 {
		u.logBuf = u.logBuf[len(u.logBuf)-300:]
	}
	u.logLabel.SetText(strings.Join(u.logBuf, "\n"))
	u.logScroll.ScrollToBottom()
}

func (u *UI) fetchExitIP() {
	c := &http.Client{Timeout: 8 * time.Second}
	ip := "não foi possível consultar"
	if resp, err := c.Get("https://api.ipify.org"); err == nil {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		if s := strings.TrimSpace(string(b)); s != "" {
			ip = s
		}
	}
	fyne.Do(func() {
		if u.state == stConnected {
			u.ipLabel.SetText(ip)
		}
	})
}

// ---------- configurações ----------

func (u *UI) showSettings(firstRun bool) {
	relay := widget.NewEntry()
	relay.SetPlaceHolder("https://meu-relay.up.railway.app")
	relay.SetText(u.cfg.RelayURL)
	relay.Validator = validateRelayURL

	user := widget.NewEntry()
	user.SetPlaceHolder("ex.: evoproxy")
	user.SetText(u.cfg.User)
	user.Validator = validateNotEmpty("o usuário")

	pass := widget.NewPasswordEntry()
	pass.SetText(u.cfg.Password)
	pass.Validator = validateNotEmpty("a senha")

	port := widget.NewEntry()
	port.SetText(strconv.Itoa(u.cfg.Port))
	port.Validator = validatePort

	autoConnect := widget.NewCheck("Conectar automaticamente ao abrir o app", nil)
	autoConnect.SetChecked(u.cfg.AutoConnect || firstRun)
	autoStart := widget.NewCheck("Abrir junto com o sistema (na bandeja)", nil)
	autoStart.SetChecked(u.cfg.AutoStart)

	items := []*widget.FormItem{
		{Text: "Endereço do relay", Widget: relay, HintText: "Domínio público do serviço relay no Railway"},
		{Text: "Usuário", Widget: user, HintText: "O mesmo da variável AUTH do relay (usuario:senha)"},
		{Text: "Senha", Widget: pass},
		{Text: "Porta do túnel", Widget: port, HintText: "Uma por aparelho. O relay usa na ordem: 1081 TV box, 1082 PC, 1083 celular…"},
		{Text: "", Widget: autoConnect},
		{Text: "", Widget: autoStart},
	}

	title := "Configurações"
	if firstRun {
		title = "Bem-vindo! Configure a conexão"
	}
	d := dialog.NewForm(title, "Salvar", "Cancelar", items, func(ok bool) {
		if !ok {
			return
		}
		portNum, _ := strconv.Atoi(strings.TrimSpace(port.Text))
		newCfg := Config{
			RelayURL:    normalizeRelayURL(relay.Text),
			User:        strings.TrimSpace(user.Text),
			Password:    pass.Text,
			Port:        portNum,
			AutoConnect: autoConnect.Checked,
			AutoStart:   autoStart.Checked,
		}
		if newCfg.AutoStart != u.cfg.AutoStart {
			if err := setAutostart(newCfg.AutoStart); err != nil {
				newCfg.AutoStart = false
				dialog.ShowError(err, u.win)
			}
		}
		changed := newCfg.RelayURL != u.cfg.RelayURL || newCfg.User != u.cfg.User ||
			newCfg.Password != u.cfg.Password || newCfg.Port != u.cfg.Port
		u.cfg = newCfg
		saveConfig(u.app.Preferences(), newCfg)
		u.refreshInfo()
		u.appendLog("", "Configurações salvas ("+hostOf(newCfg.RelayURL)+")")

		switch {
		case u.tunnel.Running() && changed:
			u.appendLog("", "Reconectando com os novos dados…")
			u.connect()
		case !u.tunnel.Running() && (firstRun || u.state == stNotConfigured || u.state == stError):
			u.connect()
		case !u.tunnel.Running():
			u.setState(stDisconnected, "")
		}
	}, u.win)
	d.Resize(fyne.NewSize(500, 0))
	d.Show()
}

func hostOf(raw string) string {
	if p, err := url.Parse(raw); err == nil && p.Host != "" {
		return p.Host
	}
	return raw
}
