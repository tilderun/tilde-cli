# Tilde CLI

## Build & Test

```
go build ./cmd/tilde/
go test ./...
go vet ./...
```

## Conventions

- In user-facing output (error messages, help text, usage strings, flag descriptions), always use the full words `organization` and `repository` — never the shorthands `org` or `repo`.
