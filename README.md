# Go rAthena Control Panel

[![Go Version](https://img.shields.io/github/go-mod/go-version/hayakawakaki/go-racp)](https://go.dev/)
[![Tests](https://img.shields.io/github/actions/workflow/status/hayakawakaki/go-racp/test.yml?branch=master&label=tests)](https://github.com/hayakawakaki/go-racp/actions/workflows/test.yml)
[![Lint](https://img.shields.io/github/actions/workflow/status/hayakawakaki/go-racp/lint.yml?branch=master&label=lint)](https://github.com/hayakawakaki/go-racp/actions/workflows/lint.yml)
[![Vulncheck](https://img.shields.io/github/actions/workflow/status/hayakawakaki/go-racp/govulncheck.yml?branch=master&label=govulncheck)](https://github.com/hayakawakaki/go-racp/actions/workflows/govulncheck.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayakawakaki/go-racp)](https://goreportcard.com/report/github.com/hayakawakaki/go-racp)

A single-binary web control panel for [rAthena](https://github.com/rathena/rathena). A self-hosted Go binary that ships with the theme, auto-migrations, and easy-configuration written in.

The control panel builds to one statically linked binary with no runtime template compilation and no separate asset pipeline to deploy.

---

## Quick start with Docker

The development stack runs the app, both databases, and a mail catcher behind a single `make` target. You need Docker with the Compose plugin, plus `make`.

```bash
git clone https://github.com/hayakawakaki/go-racp.git
cd go-racp

make dev
```

That is the whole setup. `make dev` builds the dev image and starts four services. On boot the app container applies database migrations, seeds the development data, and then launches with live reload. Give it a moment on the first run while images pull and Go modules download.

Once the logs settle, open:

| URL | What it is |
| --- | --- |
| http://localhost:8080 | The control panel |
| http://localhost:8025 | Mailpit, captures every outgoing email in dev |

Useful follow-up commands:

```bash
make app-logs        # tail just the app container
make dev-down        # stop the stack
make dev-build       # rebuild the image and restart
make psql            # open a psql shell on the control-panel database
make mysql           # open a mariadb shell on the rAthena mock database
make migrate-status  # show migration state
make help            # list every target
```

Live reload is handled by [air](https://github.com/air-verse/air). Editing any `.go`, `.templ`, or `.yml` file triggers a rebuild that regenerates theme code, compiles Templ templates, rebuilds the Tailwind stylesheet, and restarts the binary. You do not run codegen by hand.

### What runs in the dev stack

| Service | Image | Role |
| --- | --- | --- |
| `app` | local `Dockerfile.dev` | The Go binary under air with hot reload |
| `db` | `postgres:17-alpine` | Control-panel store (`cp`) plus a `cp_test` database |
| `mock-ra-db` | `mariadb:lts` | Stand-in for a live rAthena server, holds `main` and `log` |
| `mailpit` | `axllent/mailpit` | Local SMTP sink with a web inbox |

Only the development stack is exercised today. A production `Dockerfile` and `docker-compose.yml` ship as a starting point, swapping Mailpit for a Postfix relay with DKIM signing, but there is no maintained deployment yet. Treat them as scaffolding to adapt, not a turnkey production setup.

Deployment is custom, so there is no prod `make` target. The production Compose file reads each `${...}` value from the process environment. A `.env` file works, but it is not the encouraged path. Prefer [Docker secrets](https://docs.docker.com/compose/how-tos/use-secrets/) or pass the variables on the command directly:

```bash
DB_HOST=db.internal DB_USER=cp DB_PASSWORD=secret DB_MAIN_NAME=ragnarok DB_LOG_NAME=log \
DB_CP_HOST=pg.internal DB_CP_USER=cp DB_CP_PASSWORD=secret DB_CP_NAME=cp \
MAIL_HOSTNAME=mail.example.com SMTP_ALLOWED_SENDER_DOMAINS=example.com \
docker compose -f docker-compose.yml up -d --build
```

See `.env.example` for the complete set of variables the binary reads.

---

## Tech stack

| Layer | Choice |
| --- | --- |
| Language | Go 1.26 |
| Primary database | PostgreSQL via `pgx` and `pgxpool` |
| rAthena gateway | MariaDB via `database/sql` and `go-sql-driver/mysql` |
| Migrations | Goose, embedded and run on startup |
| Templating | [Templ](https://templ.guide), typed Go components compiled to functions |
| Interactivity | HTMX and Alpine.js |
| Styling | Tailwind CSS v4 |
| UI components | First-party Templ library under `internal/platform/ui`, with a dev gallery |
| HTTP | `net/http` standard library with method-aware `ServeMux` routing |
| Logging | `log/slog`, structured |
| Mail | `go-mail` over SMTP |
| Payments | Stripe, PayPal, and NowPayments(Crypto) |
| Live reload | air, with Templ and themegen codegen in the pre-build step |

There is no web framework and no ORM. Routing, middleware, and database access are written directly against the standard library and the database drivers.

---

## Architecture

```
                          HTTP request
                               |
        +----------------------v----------------------+
        |  Security headers and CSP                   |
        |  Origin and referer check                   |
        |  Session resolution (opaque token cookie)   |
        |  CSRF (HMAC bound to session fingerprint)   |
        |  Per-route rate limiting                    |
        +----------------------+----------------------+
                               |
                       Plugin registry
                  (feature slices mounted on the mux)
                               |
        +--------------+-------+--------+--------------+
        | account  admin  billing  character  guild ...|
        +--------------+----------------+--------------+
                       |                |
              Postgres (cp_*)     MariaDB (rAthena)
            primary read/write    read-mostly gateway
```

### Dual-database topology

The panel talks to two database engines with asymmetric roles.

PostgreSQL is the primary read and write store for everything the control panel owns. All of its tables are prefixed `cp_` and are versioned with Goose migrations under `migrations/`. Current tables cover sessions, single-use action tokens, account and character records, support tickets, news, an audit log, metrics, login attempts, and a currency store. Access goes through `pgx` and a `pgxpool` connection pool.

MariaDB is the rAthena gateway. It is read-mostly and treated as an external system that the game server owns. The panel connects to the `main` game database and the `log` database through `database/sql`. New control-panel state never lands here. This split keeps panel data isolated from the game database and lets the panel evolve its own schema without touching rAthena.

The project reads YAML and Lua files rather than SQL Database. The `item` and `mob` slices parse rAthena's `item_db_*.yml` and `mob_db.yml` in each respective slice which resolves the configured paths, caches the parsed result in `refdata`, and watches the files so edits hot-reload in development. File locations are set in `conf/datasources.yml`.
The admin dashboard can reload these databases on demand. A refresh reparses the source files and swaps the fresh data into the running binary, so reference updates take effect without a restart.

### Vertical slices

Features are organized as vertical slices under `internal/features/<feature>`, each split into four layers.

```
internal/features/account/
  domain/     pure business types, validation, and sentinel errors
  app/        services and use cases, no HTML and no SQL strings
  infra/      repositories (postgres_*.go and mysql_*.go)
  transport/  HTTP handlers, middleware, and per-request state
```

The `domain` layer holds the types and rules with no framework imports. The `app` layer orchestrates use cases and depends only on domain interfaces. The `infra` layer implements those interfaces against Postgres or MariaDB. The `transport` layer turns HTTP requests into use-case calls and renders Templ views. Dependencies point inward, so the business logic never imports the database driver or the HTTP package.

Shipped slices:

- `account` : registration, login, sessions, email verification, password reset, in-panel currency, bans and moderation, and logs
- `admin` : staff dashboard aggregating users, purchases, guilds, economy, item and mob status, and metrics
- `billing` : purchases and payments through Stripe, PayPal and NowPayments(Crypto), including provider webhooks
- `character` : rAthena character records and detail views
- `guild` : guild listings, details, and emblems
- `item` : item database browsing, search, and detail pages, read straight from rAthena's YAML and Lua files
- `metric` : online player counts, account character and guild totals, currency and earnings summaries, and server status surfaced to the admin dashboard
- `mob` : monster database browsing, detail, and sprites, read straight from rAthena's mob YAML
- `news` : announcements and news posts
- `stall` : live player vending stalls polled from the rAthena autotrade/vendors
- `tickets` : player support tickets with a staff workflow

### Plugin registry and lifecycle

Each feature is a self-registering plugin. A feature's `plugin.go` defines an `init()` that registers the slice with a central registry. The binary pulls features in through blank imports in `cmd/plugins.go`, so adding a feature to the build is a single import line.

```go
import (
    _ "github.com/hayakawakaki/go-racp/internal/features/account"
    _ "github.com/hayakawakaki/go-racp/internal/features/billing"
    // ...
)
```

At startup `server.Start` builds the shared infrastructure (database pools, logger, mailer, config, role resolver, token manager) and calls `plugin.MountAll` to mount every registered slice onto one `http.ServeMux`. Plugins may also contribute middleware, which the server chains around the mux. The lifecycle is `init` to register, `Mount` to attach routes, and an optional `Middleware` hook for cross-cutting behavior.

### Request gating

Every route declares its access level. A route is `Public`, wrapped as `Admin.X`, or wrapped as a `Group.Action`. These are resolved against `conf/access.yml` merged with the active theme's `access.yml`. The route registry binds each gated route to its required roles and, where configured, to a dedicated rate limiter keyed per `Group.Action`. Public actions are limited by client address. Authenticated actions are limited per user.

### Theme system

The frontend is themeable through build tags rather than runtime switching. One theme is active per binary. `cmd/read_theme` reports the active theme name, and the build compiles with `-tags theme_<name>`. `cmd/themegen` generates the glue code that wires the theme into the slices before the binary is built.

A theme under `themes/<name>/` carries its own `features/` Templ components, `pages/`, optional `platform/` overrides, `static/` assets, a `theme.yml` manifest, a `config.yml`, and an `access.yml`. Because the active theme is selected at compile time, theme code is fully type-checked and there is no template-resolution cost at runtime.

**Default is the fallback.** `default` is special. themegen scans its components and emits a `Theme` interface plus a `DefaultTheme` that implements every component. Any other theme is generated as a struct that embeds `DefaultTheme` and overrides only the components it actually redefines.

```go
type ThemeBTheme struct{ DefaultTheme }

func (ThemeBTheme) Header(...) templ.Component { /* theme B override */ }
```

So if theme B is active and provides its own `header.templ` but no `store.templ`, the store view resolves through the embedded `DefaultTheme` to the default's `store.templ`. A theme only ships the components it wants to change, and everything it leaves out falls back to default automatically. A theme whose `theme.yml` declares an app-version requirement the binary does not satisfy falls back to default entirely, with a warning logged at startup.

**Custom pages.** Drop a `.templ` or `.md` file into a theme's `pages/` directory and themegen maps it to a route by its path. `pages/download.templ` serves `/download`, `pages/index.templ` serves `/`, and `pages/server-info.md` serves `/server-info`. A `.templ` page exposes a templ function named after the file. A `.md` page is rendered to HTML at startup, taking its title from front matter or the first heading. Each generated route is registered under a `ThemePages.<Name>` tag, which is the handle you gate in that theme's root `access.yml`. Adding a page and giving it a gating entry in `access.yml` is the whole flow, no handler wiring required.

Scaffold a new theme with `make new-theme name=<name>`.

### UI component library

The frontend is built from a first-party component library under `internal/platform/ui`, not a third-party kit. It ships around thirty Templ components:

- Form primitives: button, input, select, checkbox, radio, switch, textarea, field, form
- Layout and navigation: navbar, sidebar, breadcrumb, tabs, accordion, pagination, card, table
- Overlays and feedback: modal, dropdown, tooltip, toast, alert, badge, skeleton
- A bundled SVG icon set and a theme switcher
- rAthena-specific item and mob sprite renderers

Icons are drop-in. Every `.svg` under `internal/platform/ui/icons/` is embedded and registered by filename at startup, so adding an icon is just dropping the file in with no build step or manifest to edit. Render it with `@Icon("name")`, where `name` is the filename without the extension. Icons default to `size-4` and inherit color through `currentColor`, and any class you pass overrides size or color, for example `@Icon("check", "size-8 text-blue-600")`.

Styling is composed with Tailwind. All styles are completely over-rideable. Interactivity of the components is built with Alpine.js.

Every component registers itself, and in development that registry backs a live gallery at `/_dev/components`, with a detail page per component at `/_dev/components/{name}`. The gallery mounts only when `MODE=development`, so it never ships in a built binary. Themes consume these components and may override any of them through the theme system above.

### Security

Requests pass through a defined middleware chain before reaching any handler.

- Security headers and a Content-Security-Policy strict enough to run Alpine without inline-script violations.
- Origin and referer validation against a configured allowlist.
- Session resolution from an opaque token cookie, looked up against `cp_sessions`. Tokens are random and stored hashed, never as a signed payload.
- CSRF protection using an HMAC token bound to the session fingerprint. Webhook routes under `/webhooks/*` are exempt because they authenticate by provider signature instead.
- Per-route rate limiting driven by the access config.

Account security also includes login-attempt tracking with lockout and a single-use action-token manager for email verification and password resets. The CSRF secret must be a base64 value that decodes to at least 32 bytes, validated at boot.

### Background workers and observability

A lightweight worker loop runs scheduled jobs in goroutines. It sweeps expired sessions and old login-attempt rows on configurable intervals. The account slice also runs currency deposit and withdraw workers. A metrics collector samples both databases and records into `cp_metric`. It captures online player counts (total, vendor, non-vendor, and unique) with daily, weekly, monthly, and all-time peak windows, account, character, and guild totals, in-panel currency totals and purchase earnings, and login, char, map, and web server status. The admin dashboard reads these back into its overview.

The server exposes `GET /healthz`, which pings the MariaDB main and log connections and the Postgres pool. Shutdown is graceful: a signal context cancels on `SIGINT` or `SIGTERM`, the server stops accepting connections, and in-flight requests drain within a bounded timeout. HTTP timeouts and header-size limits are set explicitly on the server.

---

## Project layout

```
cmd/            main entry, plugin imports, and the theme codegen tools
server/         Start(), config loading, middleware wiring, graceful shutdown
internal/
  features/     vertical slices (account, billing, character, ...)
  platform/     cross-cutting packages (routes, security, theme, ui, worker, ...)
  infra/        database connectors (postgres, mysql) and the mailer
  testutil/     shared test helpers and fakes
themes/         build-tag themes, default theme included
migrations/     Goose migrations for the cp_ schema
conf/           split YAML configuration (app, access, auth, security, ...)
docker/         database init and seed scripts
scripts/        entrypoints and the migration runner
static/         compiled Tailwind CSS, JavaScript, and images
```

---

## Configuration

Configuration is split across two surfaces.

Environment variables is safe-loaded, missing a required env var will cause the build to fail. Look at `.env.example` for the full list of variables loaded by the binary.

`conf/*.yml` holds structured application settings, split by concern: `app.yml`, `access.yml`, `auth.yml`, `datasources.yml`, `security.yml`, `roles.yml`, and the per-feature files. The access and role files drive route gating and rate limiting. Interval values that fall outside their allowed range are clamped at startup with a warning rather than rejected.

---

## Development workflow

```bash
make dev             # start the dev stack with live reload
make migrate-up      # apply migrations to cp and cp_test
make migrate-down    # roll back one migration on cp
make lint            # golangci-lint inside the container
make test            # go test inside the container
make new-theme name=<theme name>   # scaffold a new theme
```

Lint and test run against the active theme build tag so theme-specific code is covered. The linter configuration in `.golangci.yml` is strict and broad.

---

## Testing and CI

Unit tests sit next to the code they cover and run with `make test`. Integration tests carry a `//go:build integration` tag, are excluded from the default test run, and share a dedicated `cp_test` database.

GitHub Actions enforces four gates on every change:

- `test.yml` runs the test suite.
- `lint.yml` runs golangci-lint.
- `govulncheck.yml` scans dependencies for known vulnerabilities.
- `docker-smoke.yml` builds the production image and verifies it boots and serves `/healthz`.
