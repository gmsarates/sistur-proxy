package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

// O chisel escreve os logs em os.Stderr. Trocamos o Stderr por um pipe para
// mostrar essas linhas na janela (e continuar repassando ao terminal).
var logLines = make(chan string, 512)

func startLogCapture() {
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	realStderr := os.Stderr
	os.Stderr = w
	log.SetOutput(w)
	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			fmt.Fprintln(realStderr, line)
			select {
			case logLines <- line:
			default: // UI atrasada: descarta em vez de travar o túnel
			}
		}
	}()
}

var (
	reDatePrefix = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} (\d{2}:\d{2}:\d{2}) `)
	reConnecting = regexp.MustCompile(`^client: Connecting to (.+?)\s*$`)
	reConnected  = regexp.MustCompile(`^client: Connected \(Latency (.+)\)`)
	reRetry      = regexp.MustCompile(`^client: Retrying in (.+?)\.\.\.`)
	reConnErr    = regexp.MustCompile(`^client: Connection error: (.+)$`)
)

// tunnelEvent é o que a UI precisa saber de cada linha de log.
type tunnelEvent int

const (
	evNone tunnelEvent = iota
	evConnecting
	evConnected
	evReconnecting
	evAuthFailed
)

// parseLogLine devolve o horário, a mensagem traduzida e o evento da linha.
func parseLogLine(line string) (clock, msg string, ev tunnelEvent) {
	if m := reDatePrefix.FindStringSubmatch(line); m != nil {
		clock = m[1]
		line = line[len(m[0]):]
	}
	switch {
	case reConnecting.MatchString(line):
		return clock, "Conectando a " + reConnecting.FindStringSubmatch(line)[1], evConnecting
	case reConnected.MatchString(line):
		return clock, "Conectado (latência " + reConnected.FindStringSubmatch(line)[1] + ")", evConnected
	case reRetry.MatchString(line):
		return clock, "Tentando de novo em " + reRetry.FindStringSubmatch(line)[1], evReconnecting
	case reConnErr.MatchString(line):
		return clock, "Erro de conexão: " + reConnErr.FindStringSubmatch(line)[1], evReconnecting
	case strings.HasPrefix(line, "client: Disconnected"):
		return clock, "Conexão com o relay caiu", evReconnecting
	case strings.HasPrefix(line, "client: Authentication failed"):
		return clock, "Usuário ou senha do relay incorretos", evAuthFailed
	case strings.HasPrefix(line, "client: Cancelled"):
		return clock, "Túnel encerrado", evNone
	}
	return clock, strings.TrimPrefix(line, "client: "), evNone
}
