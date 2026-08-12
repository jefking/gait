FROM node:24-alpine AS frontend-build

WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build


FROM golang:1.26-alpine AS backend-build

WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gait ./cmd/server


FROM alpine:3.23 AS runtime

RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app

COPY --from=backend-build --chown=app:app /out/gait /app/gait
COPY --from=frontend-build --chown=app:app /src/frontend/dist /app/public

ENV PORT=8080 \
    STATIC_DIR=/app/public

EXPOSE 8080

USER app

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${PORT}/api/health" >/dev/null || exit 1

ENTRYPOINT ["/app/gait"]
