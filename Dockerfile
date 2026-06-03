FROM golang:1.26.4-alpine AS builder

ARG TW_VERSION=v4.2.4
RUN apk add --no-cache libstdc++ libgcc \
    && case "$(uname -m)" in \
         x86_64)  TW_ARCH="x64-musl"   ;; \
         aarch64) TW_ARCH="arm64-musl" ;; \
         *) echo "Unsupported arch: $(uname -m)" && exit 1 ;; \
       esac \
    && cd /tmp \
    && wget -q "https://github.com/tailwindlabs/tailwindcss/releases/download/${TW_VERSION}/tailwindcss-linux-${TW_ARCH}" \
    && wget -q "https://github.com/tailwindlabs/tailwindcss/releases/download/${TW_VERSION}/sha256sums.txt" \
    && grep "tailwindcss-linux-${TW_ARCH}\$" sha256sums.txt | sha256sum -c - \
    && mv "tailwindcss-linux-${TW_ARCH}" /usr/local/bin/tailwindcss \
    && chmod +x /usr/local/bin/tailwindcss \
    && rm sha256sums.txt

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN tailwindcss -i ./static/css/tailwind.css -o ./static/css/app.css --minify \
    && ACTIVE_THEME=$(go run ./cmd/read_theme) \
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
COPY --from=builder /app/themes ./themes
COPY --from=builder /app/static/css ./static/css
COPY conf ./conf

RUN chmod +x ./goose ./scripts/*.sh \
    && chown -R app:app /home/app

USER app

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q --spider "http://localhost:${APP_PORT:-8080}/healthz" || exit 1

CMD ["./scripts/entrypoint.prod.sh"]
