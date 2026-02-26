# ── Stage 1: Build frontend ──
FROM node:22-alpine AS web-builder
RUN corepack enable
WORKDIR /web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# ── Stage 2: Build backend ──
FROM golang:1.24-alpine AS go-builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /hermit-server ./cmd/server

# ── Stage 3: Final image ──
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=go-builder /hermit-server /usr/local/bin/hermit-server
COPY --from=web-builder /web/dist /app/web

RUN mkdir -p /data
VOLUME /data

ENV STORAGE_ROOT=/data
ENV WEB_DIR=/app/web

EXPOSE 8080
ENTRYPOINT ["hermit-server"]
