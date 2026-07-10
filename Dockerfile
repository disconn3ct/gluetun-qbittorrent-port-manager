# ---------- Build ----------
FROM golang:1.25.0-alpine@sha256:f18a072054848d87a8077455f0ac8a25886f2397f88bfdd222d6fafbb5bba440 AS builder
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
