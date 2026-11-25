#!/bin/sh
# Relay do Evo Proxy (roda no Railway).
#  - Porta pública ($PORT): o app Evo Proxy conecta aqui (WebSocket/HTTPS), com AUTH.
#  - Porta $SOCKS_PORT (padrão 1080): proxy SOCKS5 com usuário e senha para a Evolution
#    usar pela rede privada (<nome-do-servico>.railway.internal:1080).
set -e

if [ -z "$AUTH" ]; then
  echo "ERRO: defina a variável AUTH no formato usuario:senha" >&2
  exit 1
fi
SOCKS_PORT="${SOCKS_PORT:-1080}"

# Login do SOCKS (o que vai no proxy da Evolution / admin do Turvia).
# SOCKS_AUTH=usuario:senha; sem ela, usa o mesmo login do AUTH.
SOCKS_LOGIN="${SOCKS_AUTH:-$AUTH}"
SOCKS_USER="${SOCKS_LOGIN%%:*}"
SOCKS_PASS="${SOCKS_LOGIN#*:}"
if [ -z "$SOCKS_USER" ] || [ -z "$SOCKS_PASS" ] || [ "$SOCKS_USER" = "$SOCKS_LOGIN" ]; then
  echo "ERRO: SOCKS_AUTH (ou AUTH) precisa estar no formato usuario:senha" >&2
  exit 1
fi
case "$SOCKS_LOGIN" in
  *" "*) echo "ERRO: usuário e senha do SOCKS não podem ter espaço" >&2; exit 1 ;;
esac
[ -n "$SOCKS_AUTH" ] || echo "Aviso: SOCKS_AUTH não definida, o SOCKS usa o mesmo login do AUTH"

# Arquivo de senhas do gost: "usuario senha" por linha (evita escapar a senha numa URL)
SECRETS=/tmp/socks-secrets
umask 077
printf '%s %s\n' "$SOCKS_USER" "$SOCKS_PASS" > "$SECRETS"

# O app abre um SOCKS sem senha em 127.0.0.1:1081 dentro deste container (túnel chisel).
# O gost expõe na rede privada (IPv4 e IPv6) um SOCKS5 que exige usuário e senha
# e repassa cada conexão para esse SOCKS do túnel.
(
  while true; do
    gost -L "socks5://:${SOCKS_PORT}?secrets=${SECRETS}" -F "socks5://127.0.0.1:1081" || true
    sleep 1
  done
) &

echo "SOCKS5 com senha para a Evolution na porta $SOCKS_PORT (rede privada), usuário: $SOCKS_USER"
# chisel lê AUTH do ambiente
exec chisel server --reverse --host 0.0.0.0 --port "${PORT:-8080}"
