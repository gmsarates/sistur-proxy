// Evo Proxy — abre um túnel SOCKS5 reverso até um relay (chisel) no Railway,
// para que a Evolution API saia para a internet pelo IP desta máquina.
package main

import (
	"flag"
	"fmt"
	"os"

	"fyne.io/fyne/v2/app"
)

const (
	appID   = "app.evoproxy"
	appName = "Evo Proxy"
)

func main() {
	minimized := flag.Bool("minimized", false, "inicia só na bandeja, sem abrir a janela")
	exportIcon := flag.String("export-icon", "", "grava o ícone do app (PNG) neste caminho e sai")
	flag.Parse()

	if *exportIcon != "" {
		if err := os.WriteFile(*exportIcon, appIconPNG(256), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	startLogCapture()

	a := app.NewWithID(appID)
	newUI(a, *minimized).run()
}
