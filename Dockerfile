# Multi-stage: o app React builda primeiro (o resultado é o que //go:embed
# precisa encontrar em internal/webui/dist — sem esse passo o `go build`
# nem compila, ver CLAUDE.md), depois o binário Go embute esse resultado.
# Imagem final não carrega node nem o toolchain do Go, só o binário.

FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
# BUILD_TAGS fica vazio por padrão: build sem dependência nenhuma, do jeito
# que o go.mod já é. Só passa "postgres" (--build-arg BUILD_TAGS=postgres)
# depois de já ter rodado `go get github.com/jackc/pgx/v5` localmente — o
# mesmo passo manual e deliberado que o README já pede fora do Docker; o
# build aqui não decide isso sozinho. CGO_ENABLED=0 porque mesmo com a tag o
# driver pgx é puro Go.
ARG BUILD_TAGS=""
RUN if [ -n "$BUILD_TAGS" ]; then \
      CGO_ENABLED=0 go build -tags "$BUILD_TAGS" -o /out/eafcbot ./cmd/eafcbot; \
    else \
      CGO_ENABLED=0 go build -o /out/eafcbot ./cmd/eafcbot; \
    fi

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/eafcbot ./eafcbot
# Tudo que o bot grava por padrão é relativo ao diretório atual —
# ".eafc-bot/" (config, cache, snapshots, relatórios; ver o comentário de
# baseDir em internal/config/config.go). Nenhuma variável de ambiente move
# TUDO isso de uma vez (EAFC_DATA_DIR só move o histórico, não o cache nem o
# config.json), então o volume do compose monta direto em cima deste
# caminho relativo — o mesmo comportamento de rodar o binário local no repo,
# sem precisar de nenhum env var extra.
VOLUME ["/app/.eafc-bot"]
EXPOSE 4173
ENTRYPOINT ["./eafcbot"]
CMD ["serve"]
