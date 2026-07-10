# ---------- Build ----------
FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app

# faster caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /app/gluetun-qbittorrent-port-manager .

# ---------- Runtime ----------
FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
ENV PUID=1000 PGID=1000 LANG=C.UTF-8 LC_ALL=C.UTF-8

WORKDIR /app

RUN addgroup -g ${PGID} appgroup && \
    adduser -D -u ${PUID} -G appgroup appuser

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/gluetun-qbittorrent-port-manager /app/gluetun-qbittorrent-port-manager

RUN chmod +x /app/gluetun-qbittorrent-port-manager && \
    chown -R appuser:appgroup /app

USER appuser

ENTRYPOINT ["/app/gluetun-qbittorrent-port-manager"]
