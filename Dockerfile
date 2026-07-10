# ---------- Build ----------
FROM golang:1.25.0-alpine AS builder
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
FROM alpine:3.20
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
