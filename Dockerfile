# Relay do Evo Proxy (para o Railway). O Railway detecta este Dockerfile sozinho.
# O app desktop NÃO usa este arquivo: ele é compilado com `go build`.
FROM alpine:3.20

ARG CHISEL_VERSION=1.10.1
RUN apk add --no-cache socat curl \
 && curl -fsSL "https://github.com/jpillora/chisel/releases/download/v${CHISEL_VERSION}/chisel_${CHISEL_VERSION}_linux_amd64.gz" \
    | gunzip > /usr/local/bin/chisel \
 && chmod +x /usr/local/bin/chisel \
 && apk del curl

COPY relay/start.sh /start.sh
RUN chmod +x /start.sh

CMD ["/start.sh"]
