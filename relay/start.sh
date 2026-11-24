#!/bin/sh
# Relay do Evo Proxy (roda no Railway).
#  - Porta pública ($PORT): o app Evo Proxy conecta aqui (WebSocket/HTTPS).
#  - Porta $SOCKS_PORT (padrão 1080): proxy SOCKS5 para a Evolution usar
#    pela rede privada (<nome-do-servico>.railway.internal:1080).
set -e

if [ -z "$AUTH" ]; then
  echo "ERRO: defina a variável AUTH no formato usuario:senha" >&2
  exit 1
fi
SOCKS_PORT="${SOCKS_PORT:-1080}"

# O app abre o SOCKS em 127.0.0.1:1081 dentro deste container.
# Aqui ele é exposto para a rede privada: IPv6+IPv4 se houver IPv6, senão só IPv4.
(
  while true; do
    socat TCP6-LISTEN:"$SOCKS_PORT",fork,reuseaddr,ipv6only=0 TCP4:127.0.0.1:1081 2>/dev/null \
      || socat TCP4-LISTEN:"$SOCKS_PORT",fork,reuseaddr TCP4:127.0.0.1:1081
    sleep 1
  done
) &

echo "SOCKS5 para a Evolution na porta $SOCKS_PORT (rede privada)"
# chisel lê AUTH do ambiente
exec chisel server --reverse --host 0.0.0.0 --port "${PORT:-8080}"
