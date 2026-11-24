#!/bin/sh
# Instala o Evo Proxy para o usuário atual (sem sudo).
set -e
DIR=$(cd "$(dirname "$0")" && pwd)
BIN="$DIR/evoproxy"
[ -f "$BIN" ] || BIN="$DIR/build/evoproxy"

mkdir -p "$HOME/.local/bin" \
         "$HOME/.local/share/applications" \
         "$HOME/.local/share/icons/hicolor/256x256/apps"

install -m 755 "$BIN" "$HOME/.local/bin/evoproxy"
"$HOME/.local/bin/evoproxy" --export-icon "$HOME/.local/share/icons/hicolor/256x256/apps/evoproxy.png"

cat > "$HOME/.local/share/applications/evoproxy.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Evo Proxy
Comment=Túnel da Evolution API pelo IP desta máquina
Exec=$HOME/.local/bin/evoproxy
Icon=evoproxy
Terminal=false
Categories=Network;
EOF

command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$HOME/.local/share/applications" || true
echo "Pronto! Procure \"Evo Proxy\" no menu de aplicativos."
