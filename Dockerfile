# Relay do Evo Proxy (para o Railway). O Railway detecta este Dockerfile sozinho.
# O app desktop NÃO usa este arquivo: ele é compilado com `go build`.
FROM alpine:3.20

ARG CHISEL_VERSION=1.10.1
ARG GOST_VERSION=2.12.0
RUN apk add --no-cache curl \
 && curl -fsSL "https://github.com/jpillora/chisel/releases/download/v${CHISEL_VERSION}/chisel_${CHISEL_VERSION}_linux_amd64.gz" \
    | gunzip > /usr/local/bin/chisel \
 && chmod +x /usr/local/bin/chisel \
 && curl -fsSL "https://github.com/ginuerzh/gost/releases/download/v${GOST_VERSION}/gost_${GOST_VERSION}_linux_amd64.tar.gz" \
    | tar xz -C /usr/local/bin gost \
 && chmod +x /usr/local/bin/gost \
 && apk del curl

COPY relay/start.sh /start.sh
RUN chmod +x /start.sh

CMD ["/start.sh"]
