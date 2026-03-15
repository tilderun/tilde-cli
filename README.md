# Tilde CLI

A command-line interface for the [Tilde](https://tilde.run) sandbox runtime. Designed to be operated by AI agents first, humans second — every command is explicit, stateless, and returns structured output suitable for programmatic consumption.

## Installation

### From release binaries

Download the latest binary for your platform from the [Releases](https://github.com/tilderun/tilde-cli/releases) page.

### From source

```bash
go install github.com/tilderun/tilde-cli/cmd/tilde@latest
```

## Configuration

The CLI is configured entirely via environment variables:

| Variable | Default | Description |
|---|---|---|
| `TILDE_API_KEY` | *(required)* | API key (must start with `tuk-`, `trk-`, or `tak-`) |
| `TILDE_ENDPOINT_URL` | `https://tilde.run` | Base URL for the Tilde API |

```bash
export TILDE_API_KEY=tuk-your-key-here
```

## Commands

### Run a sandbox

```bash
# Run a command in a sandbox and stream output
tilde sandbox run -r organization/repository --image alpine -- echo hello

# Run in detached mode (print sandbox ID and exit)
tilde sandbox run -r organization/repository --image alpine -d -- echo hello

# Run with environment variables and a timeout
tilde sandbox run -r organization/repository --image alpine -e FOO=bar --timeout 300 -- ./script.sh
```

### Interactive shell

```bash
# Start an interactive shell
tilde shell organization/repository

# Start with a specific image
tilde shell organization/repository --image ubuntu:latest

# Run a specific command interactively
tilde shell organization/repository -- /bin/sh -l
```

### Execute a command

```bash
# Run a command non-interactively, stream output, exit with sandbox's exit code
tilde exec organization/repository -- ls -la

# With a custom image
tilde exec organization/repository --image python:3.12 -- python script.py
```

### Sandbox management

```bash
# View sandbox logs
tilde sandbox logs -r organization/repository SANDBOX_ID

# Follow logs
tilde sandbox logs -f -r organization/repository SANDBOX_ID

# Get sandbox details
tilde sandbox info -r organization/repository SANDBOX_ID
```

### List repositories

```bash
# List all accessible repositories
tilde repository ls

# List repositories in a specific organization
tilde repository ls my-organization
```

## Agent Usage

This CLI is optimized for non-interactive, automated use:

- **No prompts** — all required inputs are flags or arguments; missing values produce immediate errors
- **Structured errors** — API errors include the HTTP status, error code, and request ID for debugging
- **Exit codes** — `0` for success, non-zero mirrors the sandbox exit code
- **Stdout/stderr separation** — sandbox output goes to stdout, errors go to stderr

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
