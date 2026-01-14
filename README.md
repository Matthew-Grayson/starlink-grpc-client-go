# starlink-grpc-client-go

Go client utilities for querying a local Starlink dish gRPC endpoint (typically `192.168.100.1:9200`) using **pre-generated protobuf bindings** from:

- `github.com/starlink-community/starlink-grpc-go/pkg/spacex.com/api/device` (MIT-licensed)

**Short term:** provide a small, type-safe Go library and CLI for common dish queries (starting with `get_status`).

**Migration plan:** keep the public Go API stable while allowing the underlying protobuf source to be replaced later (e.g., with freshly generated bindings or a reflection-based transport) to handle firmware/schema drift.

---

## What this project does

- Connects to the Starlink dish gRPC server on your LAN
- Calls `SpaceX.API.Device.Device/Handle` with a `Request` oneof (e.g., `get_status`)
- Parses the `Response` into a stable, user-friendly output (CLI and/or Go API)

---

## Requirements

- Go 1.21+ (recommended: Go 1.22)
- Network access to the dish (most commonly via the Starlink router LAN)
- No TLS: the dish endpoint is typically plaintext gRPC on the local network

---

## Project structure

Designed for:
- a small reusable Go library (`pkg/starlink`)
- a CLI (`cmd/starlinkctl`) for proof-of-concept and debugging
- clean separation so you can later swap the protobuf implementation (migration)

```
.
├── cmd/
│   └── starlinkctl/
│       ├── main.go              # CLI entrypoint
│       └── README.md            # CLI usage examples (optional)
├── pkg/
│   └── starlink/
│       ├── client.go            # Public API (NewClient, GetStatus, etc.)
│       ├── dial.go              # gRPC dial options, timeouts
│       ├── models.go            # Stable structs returned by this library
│       └── errors.go            # Error helpers / wrapping
├── internal/
│   └── transport/
│       └── grpcdevice/
│           ├── device.go        # Thin wrapper around starlink-community bindings
│           └── mapping.go       # Map proto responses -> pkg/starlink models
├── examples/
│   └── status/
│       └── main.go              # Minimal example using pkg/starlink
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

### Why this layout

- `pkg/starlink` is the stable surface area you want to keep even if you switch protobuf sources.
- `internal/transport/grpcdevice` isolates the dependency on `starlink-community` bindings.
- Later, migration can become:
  - `internal/transport/protocgen` (your own generated `.pb.go`), or
  - `internal/transport/reflection` (dynamic descriptors via gRPC reflection),
  without breaking `pkg/starlink`.

---

## Installation

Add the dependency and tidy the module:

```bash
go get github.com/starlink-community/starlink-grpc-go@latest
go mod tidy
```

---

## Quick start (CLI)

Once you have the CLI wired up, usage should look like:

```bash
go run ./cmd/starlinkctl status
# or:
go run ./cmd/starlinkctl status --addr 192.168.100.1:9200 --timeout 3s
```

---

## Library usage (planned API)

```go
package main

import (
  "context"
  "fmt"
  "time"

  "github.com/Matthew-Grayson/starlink-grpc-client-go/pkg/starlink"
)

func main() {
  c, err := starlink.NewClient(starlink.Config{
    Addr:    "192.168.100.1:9200",
    Timeout: 3 * time.Second,
  })
  if err != nil {
    panic(err)
  }
  defer c.Close()

  ctx := context.Background()
  st, err := c.GetStatus(ctx)
  if err != nil {
    panic(err)
  }

  fmt.Println(st.SoftwareVersion, st.UptimeSeconds, st.PopPingLatencyMs)
}
```

---

## First steps (recommended order)

You already ran:

- `go mod init github.com/Matthew-Grayson/starlink-grpc-client-go`

Next:

### 1) Create directories and starter files

```bash
mkdir -p cmd/starlinkctl pkg/starlink internal/transport/grpcdevice examples/status
touch cmd/starlinkctl/main.go pkg/starlink/{client.go,models.go,errors.go,dial.go} internal/transport/grpcdevice/{device.go,mapping.go}
```

### 2) Add the protobuf bindings dependency

```bash
go get github.com/starlink-community/starlink-grpc-go@latest
go mod tidy
```

### 3) Implement a minimal transport wrapper (internal)

In `internal/transport/grpcdevice/device.go` implement:
- `Dial(addr, timeout) (*grpc.ClientConn, error)`
- `GetStatus(ctx) (*device.DishGetStatusResponse, error)` calling `DeviceClient.Handle`

Keep this package as the only place that imports:
- `github.com/starlink-community/starlink-grpc-go/pkg/spacex.com/api/device`

### 4) Define stable models in `pkg/starlink/models.go`

Define structs you want to return from your library, e.g.:

- `type Status struct { SoftwareVersion string; UptimeSeconds int64; PopPingLatencyMs float64; ... }`

### 5) Map proto -> stable models

In `internal/transport/grpcdevice/mapping.go`:
- Convert `DishGetStatusResponse` to `pkg/starlink.Status`

### 6) Implement public client in `pkg/starlink/client.go`

Expose:
- `type Client struct { ... }`
- `func NewClient(cfg Config) (*Client, error)`
- `func (c *Client) Close() error`
- `func (c *Client) GetStatus(ctx context.Context) (Status, error)`

### 7) Implement CLI `cmd/starlinkctl`

Start with:
- `starlinkctl status`
- flags: `--addr`, `--timeout`, `--json` (optional)

Wire it to the library (`pkg/starlink`) rather than directly to protos.

### 8) Add one example + basic integration test (optional early)

- `examples/status/main.go` uses the library.
- Add tests around mapping logic (pure unit tests, no dish required).

---

## Firmware/schema drift and migration plan

The `starlink-community/starlink-grpc-go` bindings are convenient but may lag behind firmware schema changes.

This repo is structured so you can later replace the protobuf layer without breaking consumers of `pkg/starlink`, for example by:
- generating updated `.pb.go` from newer `.proto` descriptors, or
- adding a reflection-based transport that discovers descriptors at runtime.

In both cases, `pkg/starlink` remains the stable API and only `internal/transport/*` changes.
