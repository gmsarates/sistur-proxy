# Evo Proxy

Faz a Evolution API (no Railway) sair para a internet **pelo IP da sua máquina**.
Abra o app e o túnel já fica conectado. Feche o app e a Evolution fica sem saída,
sem voltar para o IP do data center.

```
Evolution ──socks5 (usuário+senha)──► Relay (Railway) ◄══ túnel HTTPS ══ Evo Proxy (seu PC) ──► WhatsApp
           relay.railway.internal:1080                    sai com o IP da sua casa
```

O túnel usa o [chisel](https://github.com/jpillora/chisel) (SOCKS5 reverso sobre WebSocket).
Como a conexão parte do seu PC, funciona atrás de CGNAT e sem abrir porta no roteador.

---

## 1. Subir o relay no Railway

1. No mesmo projeto da Evolution, crie um serviço a partir deste repositório no GitHub.
   O Railway encontra o `Dockerfile` da raiz sozinho. Deixe **Root Directory** e
   **Dockerfile Path** vazios.
2. Variáveis do serviço:
   | Variável     | Valor                                  |
   |--------------|----------------------------------------|
   | `AUTH`       | `usuario:senha` do túnel (o app usa este) |
   | `SOCKS_AUTH` | `usuario:senha` do proxy SOCKS (a Evolution usa este). Sem ela, vale o mesmo do `AUTH` |
   | `SOCKS_PORT` | opcional, padrão `1080`                |

   Senhas sem espaço. O SOCKS **sempre** exige usuário e senha: a Evolution não grava
   proxy com usuário/senha em branco.
3. Em *Settings → Networking*, gere um **domínio público** (ex.: `meu-relay.up.railway.app`).
   É nele que o app conecta. A porta 1080 fica só na rede privada.
4. Anote o nome interno do serviço: `<nome>.railway.internal`.

## 2. Configurar a Evolution

Antes de ler o QR Code, configure o proxy da instância (pelo Manager ou pela API):

```bash
curl -X POST https://SUA-EVOLUTION/proxy/set/NOME_DA_INSTANCIA \
  -H "apikey: SUA_API_KEY" -H "Content-Type: application/json" \
  -d '{"enabled": true, "host": "NOME-DO-RELAY.railway.internal", "port": "1080",
       "protocol": "socks5", "username": "USUARIO", "password": "SENHA"}'
```

`USUARIO`/`SENHA` são os do `SOCKS_AUTH`. No Turvia, é o mesmo que vai no card
**Proxy das instâncias** do admin (tipo `socks5`, host `NOME-DO-RELAY.railway.internal`, porta `1080`).

Se a instância já foi conectada pelo IP do data center, desconecte, configure o proxy e leia o QR de novo.

## 3. Instalar o app (Linux)

```bash
./install.sh
```

Abra **Evo Proxy** no menu. Na primeira vez aparece a janela de configuração:

- **Endereço do relay:** o domínio público do passo 1 (`https://` é completado sozinho)
- **Usuário / Senha:** os mesmos da variável `AUTH`
- **Conectar automaticamente ao abrir** e **Abrir junto com o sistema**

Os dados ficam salvos pelo próprio app em `~/.config/fyne/app.evoproxy/preferences.json`.
Não precisa editar esse arquivo. A senha fica em texto puro nele, protegida só pelas
permissões da sua pasta de usuário.

Fechar a janela deixa o app rodando na bandeja (ícone com bolinha verde/amarela/vermelha).
Para sair de vez: bandeja → **Sair**.

> **GNOME:** a bandeja precisa da extensão *AppIndicator* (o Ubuntu já vem com ela).
> Sem bandeja, o app avisa no log e fechar a janela encerra o túnel.

---

## Compilar

Requisitos: Go 1.24+, gcc e as bibliotecas de desenvolvimento do OpenGL/X11.

```bash
# Ubuntu/Debian
sudo apt install golang gcc libgl1-mesa-dev xorg-dev
go build -trimpath -ldflags "-s -w" -o build/evoproxy .
```

O binário pronto precisa de glibc 2.34+ (Ubuntu 22.04, Debian 12, Fedora 35 ou mais novos).
Se ele reclamar de `GLIBC`, compile localmente com o comando acima.

### Windows e macOS (depois)

O código é o mesmo. O jeito mais fácil é o [fyne-cross](https://github.com/fyne-io/fyne-cross)
(usa Docker), ou compilar com `go build` direto em cada sistema.
Pendente: a opção "Abrir junto com o sistema" só está implementada no Linux
(`autostart_other.go` é o lugar para adicionar Windows e macOS).

## Arquivos

| Arquivo | O que faz |
|---|---|
| `main.go` | entrada do programa (`--minimized` inicia só na bandeja) |
| `ui.go` | janela, modal de configurações, bandeja e status |
| `tunnel.go` | liga/desliga o cliente chisel (reconecta sozinho) |
| `logcapture.go` | mostra o log do chisel na janela, traduzido |
| `config.go` | dados salvos e validação do formulário |
| `autostart_*.go` | abrir junto com o sistema |
| `icons.go` | ícones desenhados em código |
| `relay/` | script do relay para o Railway: chisel (túnel) + gost (SOCKS5 com senha) |
