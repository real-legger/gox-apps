# wallet

Go/Goose microservice for static file and dynamic SVG asset management.

## Overview

## Endpoints

| Method | Path | Description  |
| ------ | ---- | ------------ |
| GET    | `/`  | Health check |

## Entities

- **File** — type, bucket, name, tags, raw SVG content, status

## Environment Variables

| Variable       | Default    | Description                      |
| -------------- | ---------- | -------------------------------- |
| `DATABASE_URL` | —          | Postgres/MySQL connection string |
| `JWT_SECRET`   | `changeme` | JWT signing secret               |

## Module

```go
import "github.com/real-legger/gox-apps-wallet/app"

app.AppModule{}
```

## Development

```bash
go test ./...
go build ./...
```
