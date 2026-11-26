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

# Portas do túnel, em ordem de prioridade: cada aparelho conectado (app do PC, TV box...)
# abre o seu SOCKS em 127.0.0.1:<porta> aqui dentro. O gost usa sempre o primeiro que
# estiver disponível; se ele cair, passa para o próximo. Padrão: 1081 a 1090 (TV box = 1081,
# PC = 1082, celular = 1083, outros 1084...). Porta sem aparelho é pulada.
TUNNEL_PORTS="${TUNNEL_PORTS:-1081,1082,1083,1084,1085,1086,1087,1088,1089,1090}"
case "$TUNNEL_PORTS" in
  *[!0-9,]*|,*|*,|*,,*) echo "ERRO: TUNNEL_PORTS deve ser uma lista de portas separadas por vírgula, ex.: 1081,1082" >&2; exit 1 ;;
esac
FIRST_PORT="${TUNNEL_PORTS%%,*}"
TUNNEL_IPS=""
TUNNEL_COUNT=0
OLD_IFS="$IFS"; IFS=','
for p in $TUNNEL_PORTS; do
  TUNNEL_IPS="${TUNNEL_IPS:+$TUNNEL_IPS,}127.0.0.1:$p"
  TUNNEL_COUNT=$((TUNNEL_COUNT + 1))
done
IFS="$OLD_IFS"

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

# Cada aparelho abre um SOCKS sem senha em 127.0.0.1:<porta> dentro deste container (túnel chisel).
# O gost expõe na rede privada (IPv4 e IPv6) um SOCKS5 que exige usuário e senha
# e repassa cada conexão para o primeiro aparelho disponível (strategy=fifo).
# Um aparelho cuja porta não responde fica de fora por 10s (max_fails/fail_timeout) e a
# mesma conexão é tentada no próximo da lista ("Retries", que o gost só lê de arquivo -C;
# uma tentativa por porta). Sem nenhum aparelho, a conexão falha: nunca sai pelo IP do
# data center.
GOST_DEBUG_FLAG=""
[ "$GOST_DEBUG" = "1" ] && GOST_DEBUG_FLAG="-D"
GOST_CONFIG=/tmp/gost.json
printf '{"Retries": %s}\n' "$TUNNEL_COUNT" > "$GOST_CONFIG"
(
  while true; do
    gost $GOST_DEBUG_FLAG -C "$GOST_CONFIG" \
      -L "socks5://:${SOCKS_PORT}?secrets=${SECRETS}" \
      -F "socks5://127.0.0.1:${FIRST_PORT}?ip=${TUNNEL_IPS}&strategy=fifo&max_fails=1&fail_timeout=10s" || true
    sleep 1
  done
) &

echo "SOCKS5 com senha para a Evolution na porta $SOCKS_PORT (rede privada), usuário: $SOCKS_USER"
echo "Aparelhos do túnel, em ordem de prioridade: $TUNNEL_IPS ($TUNNEL_COUNT tentativas por conexão)"
# chisel lê AUTH do ambiente
exec chisel server --reverse --host 0.0.0.0 --port "${PORT:-8080}"
