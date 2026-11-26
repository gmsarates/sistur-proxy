#!/data/data/com.termux/files/usr/bin/bash
# Instala o Evo Proxy (TV box) no Termux e configura para ligar junto com o Android.
set -e
DIR=$(cd "$(dirname "$0")" && pwd)

pkg install -y python >/dev/null

# Instala o comando `evoproxy-tv` (com o caminho do Python do Termux no shebang)
sed "1s|.*|#!$PREFIX/bin/python3|" "$DIR/evoproxy_tv.py" > "$PREFIX/bin/evoproxy-tv"
chmod 755 "$PREFIX/bin/evoproxy-tv"

# Ligar sozinho quando a TV box ligar (precisa do app Termux:Boot).
# Cada execução fica registrada em ~/.config/evoproxy-tv/boot.log: se depois de reiniciar
# a TV box não aparecer uma linha nova lá, o Termux:Boot não rodou o script.
mkdir -p "$HOME/.termux/boot" "$HOME/.config/evoproxy-tv"
cat > "$HOME/.termux/boot/evoproxy-tv" <<EOF
#!$PREFIX/bin/sh
LOG="$HOME/.config/evoproxy-tv/boot.log"
echo "\$(date '+%d/%m %H:%M:%S') Termux:Boot rodou o script" >> "\$LOG"
termux-wake-lock
$PREFIX/bin/evoproxy-tv start >> "\$LOG" 2>&1
tail -n 50 "\$LOG" > "\$LOG.tmp" && mv "\$LOG.tmp" "\$LOG"
EOF
chmod 755 "$HOME/.termux/boot/evoproxy-tv"

echo "Instalado. Agora vamos configurar:"
echo
evoproxy-tv config
# restart: se já estava rodando (atualização), troca o processo antigo pelo novo
evoproxy-tv restart

# O script de inicialização só roda se o app Termux:Boot estiver instalado e tiver sido aberto uma vez.
echo
if command -v pm >/dev/null 2>&1 && pm list packages 2>/dev/null | grep -q '^package:com.termux.boot$'; then
  echo "Termux:Boot encontrado. Se ainda não abriu, abra o app Termux:Boot UMA vez para ele passar a rodar ao ligar."
elif command -v pm >/dev/null 2>&1 && pm list packages 2>/dev/null | grep -q '^package:com.termux$'; then
  echo "ATENÇÃO: o app Termux:Boot não está instalado. Sem ele o túnel não liga sozinho quando a TV box liga."
  echo "Instale pelo F-Droid (o mesmo lugar de onde veio o Termux) e abra uma vez."
else
  echo "Não consegui verificar se o app Termux:Boot está instalado. Ele é necessário para ligar sozinho."
fi
