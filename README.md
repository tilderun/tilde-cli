# Cerebral CLI

A command-line interface for the [Cerebral](https://cerebral.storage) data versioning API. Designed to be operated by AI agents first, humans second — every command is explicit, stateless, and returns structured output suitable for programmatic consumption.

## Installation

### From release binaries

Download the latest binary for your platform from the [Releases](https://github.com/cerebral-storage/cerebral-cli/releases) page.

### From source

```bash
go install github.com/cerebral-storage/cerebral-cli/cmd/cerebral@latest
```

## Configuration

The CLI is configured entirely via environment variables:

| Variable | Default | Description |
|---|---|---|
| `CEREBRAL_API_KEY` | *(required)* | Agent API key (must start with `cak-`) |
| `CEREBRAL_ENDPOINT_URL` | `https://cerebral.storage` | Base URL for the Cerebral API |
| `CEREBRAL_CLI_MAX_CONCURRENCY` | `16` | Max parallel upload/download workers |

```bash
export CEREBRAL_API_KEY=cak-your-key-here
```

## Workflow

All data operations in Cerebral are session-based. The typical workflow is:

1. **Start a session** — creates an isolated workspace for staging changes
2. **Upload, download, delete objects** — all mutations are staged in the session
3. **Commit** — atomically applies all staged changes and creates a new version
4. Or **rollback** — discards all staged changes

## Commands

### List repositories

```bash
# List all accessible repositories
cerebral repository ls

# List repositories in a specific organization
cerebral repository ls my-organization
```

### Session management

```bash
# Start a new session
cerebral session start cb://organization/repository
# → prints session_id

# Commit a session
cerebral session commit --session SESSION_ID -m "Add training data" cb://organization/repository
# → prints commit_id (or approval URL if review is required)

# Rollback a session
cerebral session rollback --session SESSION_ID cb://organization/repository
```

### Copy objects (`cp`)

```bash
# Upload a file
cerebral cp --session SESSION_ID ./local/file.csv cb://organization/repository/data/file.csv

# Download a file
cerebral cp --session SESSION_ID cb://organization/repository/data/file.csv ./local/file.csv

# Download to stdout
cerebral cp --session SESSION_ID cb://organization/repository/data/file.csv -

# Recursive upload
cerebral cp --session SESSION_ID -r ./local/data/ cb://organization/repository/data/

# Recursive download
cerebral cp --session SESSION_ID -r cb://organization/repository/data/ ./local/data/

# Show per-file progress
cerebral cp --session SESSION_ID -v -r ./local/data/ cb://organization/repository/data/
```

### List objects (`ls`)

```bash
# List top-level objects and prefixes
cerebral ls --session SESSION_ID cb://organization/repository

# List with prefix
cerebral ls --session SESSION_ID cb://organization/repository/data/

# Recursive listing (flat, no directory grouping)
cerebral ls --session SESSION_ID -r cb://organization/repository

# Limit results
cerebral ls --session SESSION_ID --max-results 100 cb://organization/repository
```

### Delete objects (`rm`)

```bash
# Delete a single object
cerebral rm --session SESSION_ID cb://organization/repository/data/file.csv

# Delete all objects under a prefix
cerebral rm --session SESSION_ID -r cb://organization/repository/data/
```

## URI Format

All repository references use the `cb://` scheme:

```
cb://organization/repository[/path]
```

- `organization` — the Cerebral organization slug
- `repository` — the repository name
- `path` — optional object path or prefix

## Agent Usage

This CLI is optimized for non-interactive, automated use:

- **No prompts** — all required inputs are flags or arguments; missing values produce immediate errors
- **Structured errors** — API errors include the HTTP status, error code, and request ID for debugging
- **Exit codes** — `0` for success, `1` for any error
- **Stdout/stderr separation** — data goes to stdout, progress and errors go to stderr
- **Partial failure reporting** — recursive operations report per-file errors and a summary count

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
