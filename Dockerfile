FROM golang:1.27.2-alpine3.24 AS api-builder
WORKDIR /src/apps/api
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate

FROM node:22-alpine AS web-deps
WORKDIR /app
COPY apps/web/package.json apps/web/package-lock.json ./
RUN npm ci

FROM node:22-alpine AS web-builder
WORKDIR /app
COPY --from=web-deps /app/node_modules ./node_modules
COPY apps/web/ ./
ENV API_INTERNAL_URL=http://127.0.0.1:8081
RUN npm run build

FROM node:22-alpine
RUN apk add --no-cache ca-certificates tzdata \
 && mkdir -p /app/media \
 && chown node:node /app/media
WORKDIR /app
COPY --from=api-builder /out/ /app/bin/
COPY apps/api/migrations/ /app/migrations/
COPY --from=web-builder /app/.next/standalone /app/web/
COPY --from=web-builder /app/.next/static /app/web/.next/static/
COPY deployments/start.mjs /app/start.mjs
USER node
ENV NODE_ENV=production PORT=8080 HOSTNAME=0.0.0.0 \
    API_INTERNAL_URL=http://127.0.0.1:8081 \
    MIGRATIONS_DIR=/app/migrations MEDIA_DIR=/app/media
EXPOSE 8080
CMD ["node", "/app/start.mjs"]
