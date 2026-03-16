# Tilde CLI

A command-line interface for running sandboxed commands on the [Tilde](https://tilde.run) runtime.

## Installation

### From release binaries

Download the latest binary for your platform from the [Releases](https://github.com/tilderun/tilde-cli/releases) page.

### From source

```bash
go install github.com/tilderun/tilde-cli/cmd/tilde@latest
```

## Configuration

Set your API key to get started:

```bash
export TILDE_API_KEY=tuk-your-key-here
```

| Variable | Default | Description |
|---|---|---|
| `TILDE_API_KEY` | *(required)* | Your Tilde API key |
| `TILDE_ENDPOINT_URL` | `https://tilde.run` | Base URL for the Tilde API |

## Quick Start

### Execute a command

Use `tilde exec` to run a command in a sandbox, stream its output, and exit with the sandbox's exit code:

```bash
# Run a command and stream output
tilde exec organization/repository -- ls -la

# Use a specific container image
tilde exec organization/repository --image python:3.12 -- python script.py

# Pass environment variables and set a timeout
tilde exec organization/repository --image alpine -e FOO=bar --timeout 5m -- ./script.sh
```

**Flags:**

| Flag | Description |
|---|---|
| `--image` | Container image (default: `busybox:latest`) |
| `-e, --env` | Environment variable in `KEY=VALUE` format (repeatable) |
| `--timeout` | Sandbox timeout (`30s`, `5m`, `1h`) |

### Interactive shell

Use `tilde shell` to get a fully interactive terminal session inside a sandbox:

```bash
# Start a shell
tilde shell organization/repository

# Start with a specific image
tilde shell organization/repository --image ubuntu:latest

# Run a specific command interactively
tilde shell organization/repository -- /bin/sh -l
```

`tilde shell` supports the same `--image`, `--env`, and `--timeout` flags as `tilde exec`.

## Advanced Usage

### `tilde sandbox run`

A lower-level command with full control over sandbox lifecycle:

```bash
# Run and stream output (like exec)
tilde sandbox run -r organization/repository --image alpine -- echo hello

# Detached mode — prints the sandbox ID and exits immediately
tilde sandbox run -r organization/repository --image alpine -d -- echo hello

# Interactive mode (like shell)
tilde sandbox run -r organization/repository --image alpine -i -- /bin/sh
```

Additional flags: `-d` (detach), `-i` (interactive), `--mountpoint`, `--path-prefix`.

### Sandbox management

```bash
# View sandbox logs
tilde sandbox logs -r organization/repository SANDBOX_ID

# Follow logs in real time
tilde sandbox logs -f -r organization/repository SANDBOX_ID

# Get sandbox details (status, exit code, timestamps)
tilde sandbox info -r organization/repository SANDBOX_ID
```

### List repositories

```bash
tilde repository ls
tilde repository ls my-organization
```

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
