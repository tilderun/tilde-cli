# Tilde CLI

A command-line interface for running sandboxed commands on the [Tilde](https://tilde.run) runtime.

## Installation

### From release binaries

Download the latest binary for your platform from the [Releases](https://github.com/tilderun/tilde-cli/releases) page.

### From source

```bash
go install github.com/tilderun/tilde-cli/cmd/tilde@latest
```

## Authentication

### Log in with your browser

The simplest way to authenticate is the device-login flow. It opens a browser, asks you to confirm a one-time code, and stores the resulting token in `~/.tilde/config.yaml` (mode `0600`):

```bash
tilde auth login
```

Other auth subcommands:

```bash
# Show who you're logged in as
tilde auth status

# Remove stored credentials
tilde auth logout
```

### Use an API key directly

If you'd rather not run the login flow (CI, scripts), set an API key in the environment:

```bash
export TILDE_API_KEY=tuk-your-key-here
```

Keys must start with one of `tuk-`, `trk-`, or `tak-`.

### Inside a Tilde sandbox

When the CLI runs *inside* a Tilde sandbox, it automatically picks up the sandbox's principal — no API key required. The sidecar exposes short-lived credentials via `TILDE_SANDBOX_CREDENTIALS_URI`, and the CLI uses that to mint tokens on demand. This lets you nest sandboxes or call the API from a sandbox without provisioning a separate key.

An explicit API key (CLI flag, env var, or config file) always wins over the sandbox identity, so you can still target a different principal from inside a sandbox.

### Precedence

API key resolution: `--api-key` flag → `TILDE_API_KEY` env var → `~/.tilde/config.yaml` → sandbox metadata (`TILDE_SANDBOX_CREDENTIALS_URI`).

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `TILDE_API_KEY` | *(unset)* | API key. Required unless logged in via `tilde auth login` or running inside a sandbox |
| `TILDE_ENDPOINT_URL` | `https://tilde.run` | Base URL for the Tilde API |
| `TILDE_SANDBOX_CREDENTIALS_URI` | *(unset)* | Set by the sandbox runtime; consumed automatically |

## Quick Start

### Execute a command

Use `tilde exec` to run a command in a sandbox, stream its output, and exit with the sandbox's exit code:

```bash
# Run a command and stream output
tilde exec organization/repository -- ls -la

# Use a specific Docker image
tilde exec organization/repository --image python:3.12 -- python script.py

# Pass environment variables and set a timeout
tilde exec organization/repository --image ubuntu:22.04 -e FOO=bar --timeout 5m -- ./script.sh
```

**Flags:**

| Flag | Description |
|---|---|
| `--image` | Docker image reference (default: `ubuntu:22.04`) |
| `-e, --env` | Environment variable in `KEY=VALUE` format (repeatable) |
| `--timeout` | Sandbox timeout (`30s`, `5m`, `1h`) |

### Interactive shell

Use `tilde shell` to get a fully interactive terminal session inside a sandbox:

```bash
# Start a shell
tilde shell organization/repository

# Start with a specific image
tilde shell organization/repository --image python:3.12

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

# Cancel a running sandbox
tilde sandbox cancel -r organization/repository SANDBOX_ID
```

### List repositories

```bash
tilde repository ls
tilde repository ls my-organization
```

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
