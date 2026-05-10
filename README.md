# gox-apps

Go/Goose microservices — the Go rewrite of the `ntx-apps` NestJS monorepo.

## Overview

`gox-apps` is a Go workspace containing 1 independent microservices built with the [Goose](https://github.com/awesome-goose/goose) framework. Each service lives in `libs/<name>/` as a self-contained Go module and can be deployed standalone or composed into a larger platform.

## Services

| Service                         | Module path                                   | Purpose                                      |
| ------------------------------- | --------------------------------------------- | -------------------------------------------- |
| [wallet](libs/wallet/README.md) | `github.com/thescaffold/gox-apps/libs/wallet` | balance, transactions, and wallet management |

## Repository Structure

```
gox-apps/
├── go.work               # Go workspace linking all service modules
├── go.work.sum
├── libs/
│   ├── <service>/
│   │   ├── go.mod        # Independent Go module
│   │   ├── app/          # Goose module, controller, service, routes, DTOs
│   │   ├── migrations/   # SQL schema migrations
│   │   ├── pkg/          # Optional service-specific sub-packages
│   │   └── tests/        # Integration tests
│   └── ...
└── Makefile
```

## Getting Started

### Prerequisites

- Go 1.25+
- PostgreSQL (or SQLite for local dev)

### Build all services

```bash
go build ./...
```

### Test all services

```bash
go test ./...
```

### Run a service

Each service exposes a goose `AppModule`. Wire it into a goose `Application`:

```go
package main

import (
    "github.com/awesome-goose/goose"
    wallet "github.com/thescaffold/gox-apps/libs/wallet/app"
)

func main() {
    app := goose.New(&wallet.AppModule{})
    app.Listen(":3000")
}
```

## Environment Variables (common across services)

| Variable       | Default         | Description                         |
| -------------- | --------------- | ----------------------------------- |
| `DATABASE_URL` | —               | Postgres connection string          |
| `JWT_SECRET`   | `changeme`      | JWT signing secret                  |
| `HMAC_KEY`     | `changeme-hmac` | HMAC signing key for internal calls |
| `PORT`         | `3000`          | HTTP listen port                    |

## Architecture

Services communicate via:

- **In-process event bus** — `core/events.Bus` for same-process pub/sub
- **HTTP** — `core/http.Client` for cross-service calls (HMAC-authenticated internally)
- **Database** — each service owns its own tables; no cross-service DB joins

## Publishing a release

Each service is published as a standalone Go module versioned with
path-prefixed tags. Apps depend on `gox-packages/libs/core` (and `blobs` for
the `blobs` service), so **`gox-packages` must be tagged and pushed at the
matching version first** — otherwise `go get` of any app will fail to resolve
its dependencies.

```bash
# Prerequisite: cd ../gox-packages && make publish version=0.0.1

make publish version=0.0.1
```

This pins every lib's `require gox-packages/libs/{core,blobs}` to the new
version, updates the `go.work` replace, commits the change, pushes `main`, then
tags every service at `libs/<name>/v0.0.1` and pushes all tags.

If you need to pin a different `gox-packages` version than the apps version,
pass `core_version`:

```bash
make publish version=0.0.2 core_version=0.0.1
```

The working tree must be clean before publishing — `make publish` aborts if
there are uncommitted changes.

Consumers resolve any service independently:

```bash
go get github.com/thescaffold/gox-apps/libs/identity@v0.0.1
```

### Releasing a single service

If only one service needs a new release, invoke the tag flow manually instead
of `make publish`:

```bash
git tag libs/identity/v0.0.2
git push origin libs/identity/v0.0.2
```

### Local development before tags exist

[go.work](go.work) includes workspace-scoped `replace` directives so the
in-tree copies of `gox-packages/libs/core` and `gox-packages/libs/blobs` (in
the sibling `../gox-packages` checkout) are used during dev. These directives
live only in `go.work` and are not seen by consumers of the published modules
— published `go.mod` files stay clean.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
