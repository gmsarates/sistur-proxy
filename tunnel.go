package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	chclient "github.com/jpillora/chisel/client"
)

// Tunnel controla o cliente chisel. O relay recebe uma porta SOCKS5 (na rede
// privada do Railway) e cada conexão feita nela sai por esta máquina.
// O SOCKS fica só no localhost do relay, na porta do túnel deste aparelho (cfg.Port);
// o relay expõe para a Evolution, na rede privada do Railway, o primeiro aparelho
// disponível na ordem de TUNNEL_PORTS. Ver relay/start.sh.
func tunnelRemote(port int) string {
	return fmt.Sprintf("R:127.0.0.1:%d:socks", port)
}

type Tunnel struct {
	mu     sync.Mutex
	client *chclient.Client
	cancel context.CancelFunc
}

func (t *Tunnel) Running() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.client != nil
}

// Start abre o túnel. onExit é chamado se o túnel terminar sozinho.
func (t *Tunnel) Start(cfg Config, onExit func(error)) error {
	t.Stop()

	c, err := chclient.NewClient(&chclient.Config{
		Server:           cfg.RelayURL,
		Auth:             cfg.User + ":" + cfg.Password,
		Remotes:          []string{tunnelRemote(cfg.Port)},
		KeepAlive:        25 * time.Second,
		MaxRetryCount:    -1, // tenta para sempre
		MaxRetryInterval: 15 * time.Second,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := c.Start(ctx); err != nil {
		cancel()
		return err
	}

	t.mu.Lock()
	t.client, t.cancel = c, cancel
	t.mu.Unlock()

	go func() {
		err := c.Wait()
		t.mu.Lock()
		mine := t.client == c
		if mine {
			t.client, t.cancel = nil, nil
		}
		t.mu.Unlock()
		if mine && onExit != nil { // só avisa se não foi um Stop() nosso
			onExit(err)
		}
	}()
	return nil
}

func (t *Tunnel) Stop() {
	t.mu.Lock()
	c, cancel := t.client, t.cancel
	t.client, t.cancel = nil, nil
	t.mu.Unlock()
	if c != nil {
		cancel()
		_ = c.Close()
	}
}
