FROM golang:1.26.3-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN ACTIVE_THEME=$(go run ./cmd/read_theme) \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -tags "theme_${ACTIVE_THEME}" -o main ./cmd \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/goose github.com/pressly/goose/v3/cmd/goose

FROM alpine:3.23
RUN apk --no-cache add ca-certificates \
    && adduser -D -u 10001 app
WORKDIR /home/app

COPY --from=builder /app/main .
COPY --from=builder /out/goose .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/scripts ./scripts
COPY conf ./conf

RUN chmod +x ./goose ./scripts/*.sh \
    && chown -R app:app /home/app

USER app

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q --spider "http://localhost:${APP_PORT:-8080}/healthz" || exit 1

CMD ["./scripts/entrypoint.prod.sh"]
