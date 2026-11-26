# Evo Proxy para TV box (Termux)

O mesmo túnel do app de PC, para rodar numa TV box (ou celular) Android sempre ligada.
Usa o mesmo relay e o mesmo login (`AUTH`) do app de PC.

## Instalar

1. Instale o **Termux** e o **Termux:Boot** pelo [F-Droid](https://f-droid.org)
   (as versões da Play Store estão desatualizadas). Abra o Termux:Boot uma vez para ele se registrar.
2. No Termux:
   ```bash
   pkg install -y git
   git clone https://github.com/gmsarates/sistur-proxy
   bash sistur-proxy/tvbox/install-termux.sh
   ```
3. Ele pergunta o endereço do relay, o usuário e a senha (os mesmos do app de PC) e a porta,
   e já liga o túnel.
4. Nas configurações do Android, **desative a otimização de bateria** do Termux e do Termux:Boot,
   senão o Android pode fechar o túnel com a tela desligada.

## Usar

| Comando | O que faz |
|---|---|
| `evoproxy-tv status` | mostra se está conectado, há quanto tempo e o IP de saída |
| `evoproxy-tv log` | acompanha o log (Ctrl+C para sair) |
| `evoproxy-tv stop` / `start` | desliga / liga em segundo plano |
| `evoproxy-tv restart` | religa (use depois de atualizar o script) |
| `evoproxy-tv config` | muda relay, usuário, senha ou porta |

Ele liga sozinho quando a TV box liga (Termux:Boot), reconecta sozinho se a internet ou o relay cair
e impede o Android de suspender o Termux (`termux-wake-lock`).

## Atualizar

```bash
cd sistur-proxy && git pull
bash tvbox/install-termux.sh   # Enter em cada pergunta mantém os dados atuais
```
O instalador religa o túnel com a versão nova. No `evoproxy-tv log`, a primeira linha mostra a versão em uso.

## Vários aparelhos ao mesmo tempo

Cada aparelho conectado ao relay usa uma **porta do túnel** própria: o app do PC usa a `1081`, a primeira
TV box a `1082` (padrão do `evoproxy-tv config`) e os próximos aparelhos `1083`, `1084`… até `1090`.
Todos ficam conectados ao mesmo tempo e o relay manda as conexões da Evolution para o **primeiro disponível**,
na ordem da variável `TUNNEL_PORTS` do relay (padrão `1081` a `1090`). Se um cair, o próximo assume na
mesma conexão.

- Para mudar a prioridade (ex.: TV box antes do PC), defina no relay `TUNNEL_PORTS=1082,1081,1083,...`.
- Mais de 10 aparelhos: acrescente as portas em `TUNNEL_PORTS`.
- Dois aparelhos **não podem** usar a mesma porta: o segundo fica "Esperando a porta liberar".
- Aparelhos na mesma casa saem pelo **mesmo IP** — para o WhatsApp tanto faz qual deles está ativo.

## Detalhes

- Só usa a biblioteca padrão do Python. Na primeira vez, baixa o
  [chisel](https://github.com/jpillora/chisel) 1.10.1 para o processador da TV box e confere o checksum oficial.
- Configuração em `~/.config/evoproxy-tv/config.json` (só o seu usuário lê). A senha vai para o chisel por
  variável de ambiente, não aparece na lista de processos.
