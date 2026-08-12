FROM node:24-alpine AS frontend-build

WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build


FROM golang:1.26-alpine AS backend-build

ARG GIT_CHANGES_BY_DAY_VERSION=87ad8a8d0d770a120079a439cf3e9ab205c8456d

WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download
RUN GOBIN=/out go install github.com/moltenbot000/git-changes-by-day@${GIT_CHANGES_BY_DAY_VERSION}

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gait ./cmd/server


FROM alpine:3.23 AS runtime

RUN apk add --no-cache ca-certificates git \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /app/tmp \
    && chown app:app /app/tmp \
    && chmod 0700 /app/tmp

WORKDIR /app

COPY --from=backend-build --chown=app:app /out/gait /app/gait
COPY --from=backend-build --chown=app:app /out/git-changes-by-day /usr/local/bin/git-changes-by-day
COPY --from=frontend-build --chown=app:app /src/frontend/dist /app/public

ENV PORT=8080 \
    STATIC_DIR=/app/public \
    TMPDIR=/app/tmp

EXPOSE 8080

USER app

RUN git --version \
    && test -x /usr/local/bin/git-changes-by-day \
    && test -w "${TMPDIR}"

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${PORT}/api/health" >/dev/null || exit 1

ENTRYPOINT ["/app/gait"]
