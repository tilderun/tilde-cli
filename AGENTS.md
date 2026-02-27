# Cerebral CLI — Agent Guide

## Project Overview

This is a Go CLI (`cerebral`) for the Cerebral data versioning API. It provides session-based workflows: open a session, upload/download/delete/list objects, then commit or rollback.

## API Specification

The full OpenAPI 3.0 specification lives at `api/openapi.yaml`. The CLI only uses a subset of the available endpoints — specifically sessions and objects.

## Project Layout

```
cmd/cerebral/main.go              # Entry point — calls cmd.Execute()
pkg/
  cmd/
    root.go                       # Root cobra command, env config, signal handling
    repository.go                 # repository ls
    session.go                    # session start / commit / rollback
    cp.go                         # cp (upload + download, single + recursive)
    rm.go                         # rm (single + recursive bulk delete)
    ls.go                         # ls (paginated listing)
  api/
    client.go                     # HTTP client: auth, base URL, redirect handling
    repositories.go               # Repository API methods (list)
    sessions.go                   # Session API methods (create, commit, rollback)
    objects.go                    # Object API methods (get, stage, finalize, list, delete, bulk delete)
    types.go                      # Request/response structs matching the OpenAPI spec
    errors.go                     # APIError type, parsing, helpers
  uri/
    uri.go                        # cb:// URI parser → {Org, Repo, Path}
  upload/
    upload.go                     # Presigned upload pipeline: stage → S3 PUT → finalize
  download/
    download.go                   # Presigned download: GET presigned URL → download from S3
  pool/
    pool.go                       # Bounded concurrency worker pool with fail-fast
api/
  openapi.yaml                    # Full Cerebral API specification
```

## Key Architecture Decisions

1. **Presigned uploads**: `stage → S3 PUT → finalize` — avoids proxying file data through the API server.
2. **Presigned downloads**: `GET with presign=true → 307 redirect` — the API client captures the redirect and the S3 client follows it.
3. **Two HTTP clients**: The API client disables redirect following (to capture 307 presigned URLs). The S3 client has default redirect policy and no timeout (for large files).
4. **Worker pool**: Channel-based semaphore with context cancellation for fail-fast behavior on errors.
5. **Global state**: `apiClient` and `maxConcurrency` are package-level vars in `pkg/cmd`, initialized in `PersistentPreRunE`.
6. **Session-not-found hint**: When the API returns a 404 with "session not found", the error message includes a hint to create a new session.

## Commands

| Command | Description |
|---|---|
| `cerebral repository ls [organization]` | List accessible repositories |
| `cerebral session start cb://organization/repository` | Start a new session |
| `cerebral session commit --session ID -m "msg" cb://organization/repository` | Commit a session |
| `cerebral session rollback --session ID cb://organization/repository` | Rollback a session |
| `cerebral cp --session ID [-v] <src> <dst>` | Upload or download (direction from URI position) |
| `cerebral cp --session ID [-v] -r <src> <dst>` | Recursive upload or download |
| `cerebral ls --session ID cb://organization/repository[/prefix]` | List objects |
| `cerebral rm --session ID cb://organization/repository/path` | Delete a single object |
| `cerebral rm --session ID -r cb://organization/repository/prefix` | Recursive bulk delete |

## Environment Variables

| Variable | Required | Default |
|---|---|---|
| `CEREBRAL_API_KEY` | Yes | — |
| `CEREBRAL_ENDPOINT_URL` | No | `https://cerebral.storage` |
| `CEREBRAL_CLI_MAX_CONCURRENCY` | No | `16` |

## Building

```bash
go build ./cmd/cerebral
```

## Testing

```bash
go test ./...          # run all tests
go test -race ./...    # with race detector
```

Tests use `net/http/httptest` to mock both the Cerebral API and S3 endpoints. No external services are required.

# Terminology

In all client facing documentation (README.md, help messages, flag names, etc) - always use full "organization" and "repository" terms, not "org" or "repo".

