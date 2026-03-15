# Tilde CLI — Agent Guide

## Project Overview

This is a Go CLI (`tilde`) for the Tilde sandbox runtime API. It provides commands to create and manage sandboxes, attach interactive terminals, stream output, and list repositories.

## API Specification

The full OpenAPI 3.0 specification lives at `api/openapi.yaml`. The CLI uses the sandbox and repository endpoints.

## Project Layout

```
cmd/tilde/main.go                 # Entry point — calls cmd.Execute()
pkg/
  cmd/
    root.go                       # Root cobra command, env config, signal handling
    repository.go                 # repository ls
    sandbox.go                    # sandbox run (parent command + run subcommand)
    sandbox_logs.go               # sandbox logs
    sandbox_info.go               # sandbox info
    shell.go                      # shell (interactive sandbox shortcut)
    exec.go                       # exec (non-interactive sandbox shortcut)
    terminal.go                   # WebSocket terminal attach (raw mode, resize, stdin/stdout)
    repo_flag.go                  # organization/repository argument parser
  api/
    client.go                     # HTTP client: auth, base URL, streaming support
    repositories.go               # Repository API methods (list)
    sandboxes.go                  # Sandbox API methods (create, get, cancel, status, stream, terminal)
    types.go                      # Request/response structs matching the OpenAPI spec
    errors.go                     # APIError type, parsing, helpers
  pool/
    pool.go                       # Bounded concurrency worker pool with fail-fast
api/
  openapi.yaml                    # Full Tilde API specification
```

## Key Architecture Decisions

1. **Two HTTP clients**: `HTTPClient` (30s timeout, no redirect following) for API calls. `StreamClient` (no timeout) for long-lived streaming connections.
2. **WebSocket terminal**: Uses `gorilla/websocket` with `NextReader()` for streaming reads without buffering entire messages. Binary frame protocol: 0x00=stdin, 0x01=data, 0x02=resize (JSON), 0x03=exit (JSON).
3. **Worker pool**: Channel-based semaphore with context cancellation for fail-fast behavior on errors.
4. **Global state**: `apiClient` is a package-level var in `pkg/cmd`, initialized in `PersistentPreRunE`.
5. **Sandbox status polling**: After streaming output, polls `GetSandboxStatus` until a terminal state (`committed`, `awaiting_approval`, `failed`, `cancelled`) is reached.
6. **Wait for running**: Interactive commands poll sandbox status until `running` before attempting WebSocket connection.

## Commands

| Command | Description |
|---|---|
| `tilde sandbox run -r organization/repository --image IMG [-- CMD...]` | Create and run a sandbox |
| `tilde sandbox logs -r organization/repository SANDBOX_ID` | Stream sandbox output |
| `tilde sandbox info -r organization/repository SANDBOX_ID` | Show sandbox details |
| `tilde shell organization/repository [-- CMD...]` | Interactive sandbox (default: busybox:latest) |
| `tilde exec organization/repository -- CMD...` | Non-interactive sandbox (default: busybox:latest) |
| `tilde repository ls [organization]` | List accessible repositories |

## Environment Variables

| Variable | Required | Default |
|---|---|---|
| `TILDE_API_KEY` | Yes | — |
| `TILDE_ENDPOINT_URL` | No | `https://tilde.run` |

## Building

```bash
go build ./cmd/tilde
```

## Testing

```bash
go test ./...          # run all tests
go test -race ./...    # with race detector
```

Tests use `net/http/httptest` to mock the Tilde API. No external services are required.

# Terminology

In all client facing documentation (README.md, help messages, flag names, etc) - always use full "organization" and "repository" terms, not "org" or "repo".
