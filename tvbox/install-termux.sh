#!/data/data/com.termux/files/usr/bin/bash
# Instala o Evo Proxy (TV box) no Termux e configura para ligar junto com o Android.
set -e
DIR=$(cd "$(dirname "$0")" && pwd)

pkg install -y python >/dev/null

# Instala o comando `evoproxy-tv` (com o caminho do Python do Termux no shebang)
sed "1s|.*|#!$PREFIX/bin/python3|" "$DIR/evoproxy_tv.py" > "$PREFIX/bin/evoproxy-tv"
chmod 755 "$PREFIX/bin/evoproxy-tv"

# Ligar sozinho quando a TV box ligar (precisa do app Termux:Boot)
mkdir -p "$HOME/.termux/boot"
cat > "$HOME/.termux/boot/evoproxy-tv" <<EOF
#!$PREFIX/bin/sh
termux-wake-lock
$PREFIX/bin/evoproxy-tv start
EOF
chmod 755 "$HOME/.termux/boot/evoproxy-tv"

echo "Instalado. Agora vamos configurar:"
echo
evoproxy-tv config
evoproxy-tv start
