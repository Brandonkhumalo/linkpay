FROM golang:1.27.2-alpine3.24 AS builder
WORKDIR /src/apps/api
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate \
 && CGO_ENABLED=0 go build -trimpath -o /out/healthcheck ./cmd/healthcheck

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata && addgroup -S linkpay && adduser -S -G linkpay linkpay && mkdir -p /app/media && chown linkpay:linkpay /app/media
WORKDIR /app
COPY --from=builder /out/ /app/bin/
COPY apps/api/migrations/ /app/migrations/
USER linkpay
ENV PORT=8080 MIGRATIONS_DIR=/app/migrations MEDIA_DIR=/app/media
EXPOSE 8080
CMD ["/app/bin/server"]
