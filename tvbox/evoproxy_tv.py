#!/usr/bin/env python3
"""Evo Proxy para TV box / Android (Termux) — o mesmo túnel do app de PC, sem interface gráfica.

Conecta ao relay (chisel) e faz a Evolution sair para a internet pelo IP desta casa.
Só usa a biblioteca padrão do Python.

Comandos:
  evoproxy-tv config   pergunta os dados do relay e salva
  evoproxy-tv start    liga em segundo plano (continua rodando com a tela do Termux fechada)
  evoproxy-tv stop     desliga
  evoproxy-tv restart  religa (use depois de atualizar o script)
  evoproxy-tv status   mostra se está conectado e por qual IP
  evoproxy-tv log      acompanha o log (Ctrl+C para sair)
  evoproxy-tv run      roda em primeiro plano (útil para testar)
"""

import argparse
import asyncio
import errno
import getpass
import gzip
import hashlib
import json
import os
import platform
import re
import shutil
import signal
import socket
import stat
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

APP = "evoproxy-tv"
VERSION = "2026.11.25-3"
CHISEL_VERSION = "1.10.1"
# Checksums oficiais (chisel_1.10.1_checksums.txt do GitHub do chisel).
CHISEL_SHA256 = {
    "linux_arm64": "f55beb68fb99b69903df1adcff4197fbfdb82cb0ee596848c0f055dc219da983",
    "linux_armv7": "f81c1497d22caa0bb6e461b36e9f46a27f9edeaae3e3994aae2f8a5c2c9b84ef",
    "linux_armv6": "bc9f6f07709af29191d77a48f451404c0d0fbff686460b3d11982ab1420fce9f",
    "linux_386": "9737179a33736ecfd3bc6c25d48c0b64285bbc7d06f6494f8650e945cf538741",
    "linux_amd64": "0525aa3c5d457f2a4075e66221d5125d434bedf15006d3271c213f5cd6ff2230",
}
DEFAULT_PORT = 1082  # porta do túnel no relay: o app do PC usa 1081, a TV box 1082
RESTART_DELAY = 5  # segundos antes de reabrir o chisel se ele fechar
AUTH_FAIL_RETRY = 60  # senha errada: tenta de novo devagar (pode ter sido trocada no relay)
DNS_RETRY = 15  # sem internet / DNS: tenta de novo a cada 15s
RERESOLVE_AFTER = 3  # erros de conexão seguidos antes de conferir se o IP do relay mudou

HOME = os.path.expanduser("~")
CONF_DIR = os.environ.get("EVOPROXY_TV_HOME") or os.path.join(HOME, ".config", APP)
CONFIG_FILE = os.path.join(CONF_DIR, "config.json")
STATE_FILE = os.path.join(CONF_DIR, "state.json")
LOG_FILE = os.path.join(CONF_DIR, "evoproxy-tv.log")
PID_FILE = os.path.join(CONF_DIR, "evoproxy-tv.pid")
BIN_DIR = os.path.join(CONF_DIR, "bin")


# ---------------------------------------------------------------- utilidades

def now():
    return time.strftime("%H:%M:%S")


def say(msg):
    print(f"{now()}  {msg}", flush=True)


def ensure_dir():
    os.makedirs(CONF_DIR, exist_ok=True)
    os.chmod(CONF_DIR, 0o700)


def write_json(path, data, private=False):
    ensure_dir()
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
    if private:
        os.chmod(tmp, 0o600)
    os.replace(tmp, path)


def read_json(path):
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except (OSError, ValueError):
        return None


def normalize_url(url):
    url = url.strip().rstrip("/")
    if url and "://" not in url:
        url = "https://" + url
    return url


def validate_url(url):
    parsed = urllib.parse.urlparse(url)
    return parsed.scheme in ("http", "https") and bool(parsed.netloc)


# ---------------------------------------------------------------- configuração

def load_config():
    cfg = read_json(CONFIG_FILE)
    if not cfg:
        return None
    if not all(cfg.get(k) for k in ("relay_url", "user", "password")):
        return None
    cfg.setdefault("port", DEFAULT_PORT)
    return cfg


def ask(prompt, default=None, secret=False):
    suffix = f" [{default}]" if default and not secret else ""
    suffix = " [manter a atual]" if default and secret else suffix
    while True:
        value = (getpass.getpass if secret else input)(f"{prompt}{suffix}: ").strip()
        if value:
            return value
        if default:
            return default


def cmd_config(_args=None):
    old = read_json(CONFIG_FILE) or {}
    print("Configuração do Evo Proxy (TV box)")
    print("Use os mesmos dados do app do PC: o domínio público do relay e o usuário/senha da variável AUTH.\n")

    while True:
        url = normalize_url(ask("Endereço do relay", old.get("relay_url")))
        if validate_url(url):
            break
        print("  Endereço inválido. Exemplo: https://meu-relay.up.railway.app")
    user = ask("Usuário", old.get("user"))
    password = ask("Senha", old.get("password"), secret=True)
    while True:
        raw = ask("Porta do túnel no relay (cada aparelho conectado ao mesmo tempo precisa de uma)",
                  str(old.get("port", DEFAULT_PORT)))
        if raw.isdigit() and 1 <= int(raw) <= 65535:
            port = int(raw)
            break
        print("  Porta inválida.")

    write_json(CONFIG_FILE, {"relay_url": url, "user": user, "password": password, "port": port},
               private=True)
    print(f"\nSalvo em {CONFIG_FILE} (só o seu usuário consegue ler).")
    if is_running():
        print("O túnel está ligado: rode `evoproxy-tv stop` e `evoproxy-tv start` para usar os novos dados.")
    else:
        print("Para ligar em segundo plano: evoproxy-tv start")


# ---------------------------------------------------------------- binário do chisel

def chisel_arch():
    machine = platform.machine().lower()
    if machine in ("aarch64", "arm64"):
        return "linux_arm64"
    if machine in ("armv7l", "armv8l", "armv7", "arm"):
        return "linux_armv7"
    if machine.startswith("armv6"):
        return "linux_armv6"
    if machine in ("i386", "i686", "x86"):
        return "linux_386"
    if machine in ("x86_64", "amd64"):
        return "linux_amd64"
    raise SystemExit(f"Processador não suportado: {machine}")


def chisel_path():
    custom = os.environ.get("EVOPROXY_TV_CHISEL")
    if custom:
        return custom
    return os.path.join(BIN_DIR, f"chisel-{CHISEL_VERSION}")


def ensure_chisel():
    path = chisel_path()
    if os.path.isfile(path) and os.access(path, os.X_OK):
        return path
    arch = chisel_arch()
    name = f"chisel_{CHISEL_VERSION}_{arch}.gz"
    url = f"https://github.com/jpillora/chisel/releases/download/v{CHISEL_VERSION}/{name}"
    say(f"Baixando o chisel {CHISEL_VERSION} ({arch})…")
    with urllib.request.urlopen(url, timeout=60) as resp:
        data = resp.read()
    digest = hashlib.sha256(data).hexdigest()
    if digest != CHISEL_SHA256[arch]:
        raise SystemExit("O arquivo baixado do chisel não confere com o checksum oficial. Abortando.")
    os.makedirs(BIN_DIR, exist_ok=True)
    tmp = path + ".tmp"
    with open(tmp, "wb") as f:
        f.write(gzip.decompress(data))
    os.chmod(tmp, os.stat(tmp).st_mode | stat.S_IXUSR)
    os.replace(tmp, path)
    say("chisel pronto (checksum conferido).")
    return path


# ---------------------------------------------------------------- DNS no Android

def needs_manual_dns():
    """O chisel oficial é um binário Go estático: ele procura o DNS em /etc/resolv.conf, que não
    existe no Android, e acaba consultando [::1]:53. Nesses casos resolvemos o nome aqui
    (o Python do Termux usa o DNS do Android) e passamos o IP ao chisel."""
    if os.environ.get("EVOPROXY_TV_FORCE_DNS") == "1":
        return True
    # Android/Termux: sempre (o resolv.conf do Android, quando existe, não serve ao Go).
    if os.environ.get("TERMUX_VERSION") or os.environ.get("ANDROID_ROOT") or "com.termux" in os.environ.get("PREFIX", ""):
        return True
    # Outros sistemas: só se não houver nenhum "nameserver" configurado (o Go cairia em [::1]:53).
    try:
        with open("/etc/resolv.conf", encoding="utf-8", errors="ignore") as f:
            return not any(line.split()[:1] == ["nameserver"] for line in f)
    except OSError:
        return True


def resolve(host):
    """Endereços do host, IPv4 primeiro."""
    infos = socket.getaddrinfo(host, None, type=socket.SOCK_STREAM)
    v4 = [i[4][0] for i in infos if i[0] == socket.AF_INET]
    v6 = [i[4][0] for i in infos if i[0] == socket.AF_INET6]
    seen = []
    for ip in v4 + v6:
        if ip not in seen:
            seen.append(ip)
    return seen


def is_ip(host):
    for family in (socket.AF_INET, socket.AF_INET6):
        try:
            socket.inet_pton(family, host)
            return True
        except (OSError, ValueError):
            pass
    return False


def chisel_target(relay_url):
    """Devolve (url para o chisel, argumentos extras, ip usado ou None).

    Quando o DNS precisa ser feito aqui, troca o nome pelo IP na URL e usa --hostname,
    que mantém o nome no cabeçalho Host e na verificação do certificado TLS."""
    parsed = urllib.parse.urlparse(relay_url)
    host = parsed.hostname or ""
    if not needs_manual_dns() or not host or is_ip(host):
        return relay_url, [], None
    ip = resolve(host)[0]
    netloc = f"[{ip}]" if ":" in ip else ip
    if parsed.port:
        netloc += f":{parsed.port}"
    return urllib.parse.urlunparse(parsed._replace(netloc=netloc)), ["--hostname", host], ip


def chisel_env(base):
    env = dict(base)
    # Certificados do Termux (o Go também acha os do Android, isto é só reforço).
    prefix = os.environ.get("PREFIX", "")
    cert = os.path.join(prefix, "etc", "tls", "cert.pem") if prefix else ""
    if cert and os.path.isfile(cert) and "SSL_CERT_FILE" not in env:
        env["SSL_CERT_FILE"] = cert
    return env


# ---------------------------------------------------------------- SOCKS5 local (Android)
#
# A Evolution manda os nomes (ex.: web.whatsapp.com) para o proxy resolver. No Android, o SOCKS
# interno do chisel resolveria com o DNS quebrado do Go ([::1]:53). Por isso, no Android, o chisel
# só repassa o tráfego (R:127.0.0.1:<porta>:127.0.0.1:<local>) para este SOCKS5, que resolve os
# nomes com o DNS do Android (getaddrinfo do Python) e abre as conexões. Só aceita conexões locais.

SOCKS_CONNECT_TIMEOUT = 20


def _socks_reply(code, host="0.0.0.0", port=0):
    try:
        addr = socket.inet_pton(socket.AF_INET, host)
        atyp = b"\x01"
    except OSError:
        try:
            addr = socket.inet_pton(socket.AF_INET6, host)
            atyp = b"\x04"
        except OSError:
            addr, atyp = b"\x00\x00\x00\x00", b"\x01"
    return b"\x05" + bytes([code]) + b"\x00" + atyp + addr + port.to_bytes(2, "big")


def _error_code(exc):
    if isinstance(exc, socket.gaierror):
        return 0x04  # host inalcançável (nome não encontrado)
    if isinstance(exc, (asyncio.TimeoutError, TimeoutError)):
        return 0x04
    if isinstance(exc, ConnectionRefusedError):
        return 0x05
    if isinstance(exc, OSError) and exc.errno in (errno.ENETUNREACH,):
        return 0x03
    if isinstance(exc, OSError) and exc.errno in (errno.EHOSTUNREACH,):
        return 0x04
    return 0x01


async def _pipe(reader, writer):
    try:
        while True:
            data = await reader.read(65536)
            if not data:
                break
            writer.write(data)
            await writer.drain()
    except (ConnectionError, OSError, asyncio.CancelledError):
        pass
    finally:
        try:
            if writer.can_write_eof():
                writer.write_eof()
        except (OSError, RuntimeError):
            pass


async def _socks_client(reader, writer):
    remote_writer = None
    try:
        head = await reader.readexactly(2)
        if head[0] != 5:
            return
        await reader.readexactly(head[1])  # métodos oferecidos
        writer.write(b"\x05\x00")  # sem autenticação (só escuta em 127.0.0.1)
        await writer.drain()

        ver, cmd, _, atyp = await reader.readexactly(4)
        if atyp == 1:
            host = socket.inet_ntop(socket.AF_INET, await reader.readexactly(4))
        elif atyp == 3:
            host = (await reader.readexactly((await reader.readexactly(1))[0])).decode("idna")
        elif atyp == 4:
            host = socket.inet_ntop(socket.AF_INET6, await reader.readexactly(16))
        else:
            writer.write(_socks_reply(0x08))
            return
        port = int.from_bytes(await reader.readexactly(2), "big")
        if ver != 5 or cmd != 1:  # só CONNECT
            writer.write(_socks_reply(0x07))
            return

        try:
            remote_reader, remote_writer = await asyncio.wait_for(
                asyncio.open_connection(host, port), SOCKS_CONNECT_TIMEOUT)
        except Exception as exc:  # noqa: BLE001 — qualquer falha vira resposta SOCKS
            writer.write(_socks_reply(_error_code(exc)))
            return
        bound = remote_writer.get_extra_info("sockname") or ("0.0.0.0", 0)
        writer.write(_socks_reply(0x00, bound[0], bound[1]))
        await writer.drain()
        await asyncio.gather(_pipe(reader, remote_writer), _pipe(remote_reader, writer))
    except (asyncio.IncompleteReadError, ConnectionError, OSError):
        pass
    finally:
        for w in (writer, remote_writer):
            if w is not None:
                try:
                    w.close()
                except (OSError, RuntimeError):
                    pass


class LocalSocks:
    """SOCKS5 em 127.0.0.1 numa porta livre, rodando numa thread própria."""

    def __init__(self):
        self.port = None
        self._ready = threading.Event()
        self._error = None
        threading.Thread(target=self._run, daemon=True).start()
        self._ready.wait(10)
        if self._error:
            raise self._error
        if self.port is None:
            raise RuntimeError("o SOCKS local não iniciou")

    def _run(self):
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        try:
            server = loop.run_until_complete(
                asyncio.start_server(_socks_client, "127.0.0.1", 0))
            self.port = server.sockets[0].getsockname()[1]
        except Exception as exc:  # noqa: BLE001
            self._error = exc
            self._ready.set()
            return
        self._ready.set()
        loop.run_forever()


# ---------------------------------------------------------------- tradução do log

RE_PREFIX = re.compile(r"^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} ")
EVENTS = [
    (re.compile(r"client: Connecting to (.+)"), "conectando", lambda m: f"Conectando a {m.group(1).strip()}"),
    (re.compile(r"client: Connected \(Latency (.+)\)"), "conectado", lambda m: f"Conectado (latência {m.group(1)})"),
    (re.compile(r"client: Retrying in (.+?)\.\.\."), "reconectando", lambda m: f"Tentando de novo em {m.group(1)}"),
    (re.compile(r"client: Connection error: .*cannot listen", re.I), "porta_ocupada", None),
    (re.compile(r"client: Authentication failed"), "senha_errada",
     lambda m: "Usuário ou senha do relay incorretos (rode `evoproxy-tv config`)"),
    (re.compile(r"client: Connection error: (.+)"), "reconectando", lambda m: f"Erro de conexão: {m.group(1)}"),
    (re.compile(r"client: Disconnected"), "reconectando", lambda m: "Conexão com o relay caiu"),
]


def translate(line, port):
    """Devolve (evento, mensagem) para uma linha do log do chisel."""
    text = RE_PREFIX.sub("", line.strip())
    for regex, event, fmt in EVENTS:
        m = regex.search(text)
        if m:
            if event == "porta_ocupada":
                return event, (f"A porta {port} do relay já está em uso por outro aparelho. "
                               "Vou esperar ela liberar (ou escolha outra porta com `evoproxy-tv config`).")
            return event, fmt(m)
    return None, text.replace("client: ", "", 1)


# ---------------------------------------------------------------- supervisor

class Supervisor:
    def __init__(self, cfg):
        self.cfg = cfg
        self.proc = None
        self.stopping = False
        self.state = {"estado": "iniciando", "desde": time.time(), "ip_saida": None,
                      "porta": cfg["port"], "relay": cfg["relay_url"], "mensagem": ""}

    def set_state(self, estado, mensagem=None, **extra):
        if estado != self.state["estado"]:
            self.state["desde"] = time.time()
        self.state["estado"] = estado
        if mensagem is not None:
            self.state["mensagem"] = mensagem
        self.state.update(extra)
        self.state["pid"] = os.getpid()
        write_json(STATE_FILE, self.state)

    def fetch_exit_ip(self):
        try:
            with urllib.request.urlopen("https://api.ipify.org", timeout=10) as r:
                ip = r.read(64).decode().strip()
        except (urllib.error.URLError, OSError):
            ip = None
        if self.state["estado"] == "conectado":
            self.set_state("conectado", ip_saida=ip)
            if ip:
                say(f"IP de saída: {ip}")

    def sleep(self, seconds):
        for _ in range(int(seconds)):
            if self.stopping:
                return
            time.sleep(1)

    def relay_ip_changed(self, current_ip):
        try:
            return current_ip not in resolve(urllib.parse.urlparse(self.cfg["relay_url"]).hostname)
        except OSError:
            return False

    def stop(self, *_):
        self.stopping = True
        if self.proc and self.proc.poll() is None:
            self.proc.terminate()

    def run(self):
        signal.signal(signal.SIGTERM, self.stop)
        signal.signal(signal.SIGINT, self.stop)
        if shutil.which("termux-wake-lock"):
            subprocess.run(["termux-wake-lock"], check=False)  # impede o Android de "dormir" o túnel
        binary = ensure_chisel()
        port = self.cfg["port"]
        say(f"Evo Proxy (TV box) v{VERSION} — relay {self.cfg['relay_url']}, porta {port}")
        if needs_manual_dns():
            local = LocalSocks()
            remote = f"R:127.0.0.1:{port}:127.0.0.1:{local.port}"
            say("DNS pelo Android: as conexões do WhatsApp passam pelo SOCKS local deste aparelho")
        else:
            remote = f"R:127.0.0.1:{port}:socks"

        while not self.stopping:
            env = chisel_env(os.environ)
            env["AUTH"] = f"{self.cfg['user']}:{self.cfg['password']}"
            try:
                url, extra, ip = chisel_target(self.cfg["relay_url"])
            except OSError as exc:
                msg = f"Sem internet ou DNS: não consegui encontrar o relay ({exc}). Tentando de novo em {DNS_RETRY}s"
                say(msg)
                self.set_state("reconectando", msg, ip_saida=None)
                self.sleep(DNS_RETRY)
                continue
            if ip:
                say(f"Relay resolvido para {ip}")
            self.set_state("conectando", "Iniciando túnel", ip_saida=None)
            self.proc = subprocess.Popen(
                [binary, "client", "--keepalive", "25s", "--max-retry-interval", "15s",
                 *extra, url, remote],
                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, env=env,
                text=True, bufsize=1,
            )
            auth_failed = False
            errors_in_a_row = 0
            for line in self.proc.stdout:
                event, msg = translate(line, port)
                if not msg:
                    continue
                say(msg)
                if event == "conectado":
                    errors_in_a_row = 0
                    self.set_state("conectado", msg)
                    threading.Thread(target=self.fetch_exit_ip, daemon=True).start()
                elif event == "porta_ocupada":
                    self.set_state("porta_ocupada", msg, ip_saida=None)
                elif event == "senha_errada":
                    auth_failed = True
                    self.set_state("senha_errada", msg, ip_saida=None)
                    self.proc.terminate()
                elif event in ("conectando", "reconectando"):
                    if self.state["estado"] != "porta_ocupada" or event == "conectando":
                        self.set_state(event, msg, ip_saida=None)
                    if ip and msg.startswith("Erro de conexão"):
                        errors_in_a_row += 1
                        if errors_in_a_row >= RERESOLVE_AFTER and self.relay_ip_changed(ip):
                            say("O IP do relay mudou; reconectando com o novo endereço")
                            self.proc.terminate()
            self.proc.wait()
            if self.stopping:
                break
            delay = AUTH_FAIL_RETRY if auth_failed else RESTART_DELAY
            if not auth_failed:
                say(f"O túnel fechou (código {self.proc.returncode}). Reabrindo em {delay}s…")
            else:
                say(f"Tentando de novo em {delay}s…")
            self.set_state("reconectando" if not auth_failed else "senha_errada")
            self.sleep(delay)

        self.set_state("desligado", "Túnel desligado", ip_saida=None)
        say("Túnel desligado.")


def cmd_run(_args=None):
    cfg = load_config()
    if not cfg:
        print("Ainda não configurado.")
        cmd_config()
        cfg = load_config()
    ensure_dir()
    with open(PID_FILE, "w") as f:
        f.write(str(os.getpid()))
    try:
        Supervisor(cfg).run()
    finally:
        try:
            if read_pid() == os.getpid():
                os.remove(PID_FILE)
        except OSError:
            pass


# ---------------------------------------------------------------- segundo plano

def read_pid():
    try:
        with open(PID_FILE) as f:
            return int(f.read().strip())
    except (OSError, ValueError):
        return None


def is_running():
    pid = read_pid()
    if not pid:
        return False
    try:
        os.kill(pid, 0)
    except OSError:
        return False
    return True


def cmd_start(_args=None):
    if is_running():
        print("Já está ligado. Veja com: evoproxy-tv status")
        return
    if not load_config():
        print("Ainda não configurado.")
        cmd_config()
    ensure_dir()
    log = open(LOG_FILE, "a", encoding="utf-8")
    subprocess.Popen([sys.executable, os.path.abspath(__file__), "run"],
                     stdout=log, stderr=subprocess.STDOUT, stdin=subprocess.DEVNULL,
                     start_new_session=True)
    for _ in range(30):
        time.sleep(0.5)
        state = read_json(STATE_FILE) or {}
        if is_running() and state.get("estado") not in (None, "desligado"):
            break
    if is_running():
        print("Ligado em segundo plano. Acompanhe com: evoproxy-tv status  (ou evoproxy-tv log)")
    else:
        print(f"Não consegui ligar. Veja o log: {LOG_FILE}")


def cmd_stop(_args=None):
    pid = read_pid()
    if not pid or not is_running():
        print("Não está ligado.")
        return
    os.kill(pid, signal.SIGTERM)
    for _ in range(20):
        if not is_running():
            break
        time.sleep(0.25)
    print("Desligado." if not is_running() else "Ainda encerrando… rode `evoproxy-tv status` em instantes.")


LABELS = {
    "conectado": "🟢 Conectado",
    "conectando": "🟡 Conectando",
    "reconectando": "🟡 Reconectando",
    "iniciando": "🟡 Iniciando",
    "porta_ocupada": "🟠 Esperando a porta liberar",
    "senha_errada": "🔴 Usuário ou senha incorretos",
    "desligado": "⚪ Desligado",
}


def cmd_restart(_args=None):
    if is_running():
        cmd_stop()
    cmd_start()


def cmd_status(_args=None):
    state = read_json(STATE_FILE) or {}
    running = is_running()
    estado = state.get("estado", "desligado") if running else "desligado"
    print(LABELS.get(estado, estado))
    if running:
        mins = int((time.time() - state.get("desde", time.time())) // 60)
        print(f"  há {mins} min · relay {state.get('relay')} · porta {state.get('porta')}")
        if state.get("ip_saida"):
            print(f"  IP de saída: {state['ip_saida']}")
        if state.get("mensagem") and estado != "conectado":
            print(f"  {state['mensagem']}")


def cmd_log(_args=None):
    if not os.path.exists(LOG_FILE):
        print("Ainda não há log.")
        return
    with open(LOG_FILE, encoding="utf-8") as f:
        lines = f.readlines()
        sys.stdout.writelines(lines[-30:])
        try:
            while True:
                line = f.readline()
                if line:
                    sys.stdout.write(line)
                    sys.stdout.flush()
                else:
                    time.sleep(0.5)
        except KeyboardInterrupt:
            pass


def main():
    parser = argparse.ArgumentParser(prog=APP, description="Túnel do Evo Proxy para TV box / Termux")
    sub = parser.add_subparsers(dest="cmd")
    for name, fn, help_ in [
        ("config", cmd_config, "configurar o relay"),
        ("start", cmd_start, "ligar em segundo plano"),
        ("stop", cmd_stop, "desligar"),
        ("restart", cmd_restart, "religar (use depois de atualizar)"),
        ("status", cmd_status, "ver o estado"),
        ("log", cmd_log, "acompanhar o log"),
        ("run", cmd_run, "rodar em primeiro plano"),
    ]:
        sub.add_parser(name, help=help_).set_defaults(fn=fn)
    args = parser.parse_args()
    if not getattr(args, "fn", None):
        parser.print_help()
        return
    args.fn(args)


if __name__ == "__main__":
    main()
