# AGENTS.md

## Quick commands

```bash
task default      # build + vet + test + lint (the full CI-equivalent check)
task build        # go build .
task test         # go test ./...
task vet          # go vet ./...
task lint         # golangci-lint (runs fmt-check first)
task lint-fix     # golangci-lint --fix
task golden       # regenerate golden testdata files
```

To run a single test:

```bash
go test ./handlers -run TestSTFiles/addresses
```

## Real-project lossless test (opt-in)

`TestProjectFormatLossless` in `handlers/project_test.go` formats every
supported file in a real ST/TwinCAT project tree and independently verifies
(without the internal lexer/parser) that only whitespace and character case
changed. It is skipped unless `STFORMAT_PROJECT_DIR` is set:

```powershell
$env:STFORMAT_PROJECT_DIR = 'C:\path\to\project'
go test ./handlers -run TestProjectFormatLossless
```

The test only reads files; it never writes back. Verification normalises
both the original and the formatted output (lowercasing every character and
stripping all whitespace) and requires the two normalised strings to be
equal, so any removed/added/reordered content fails the test.

## Build and verification order

`vet -> test -> lint` is the safe order. `task default` already runs all four steps (build, vet, test, lint).

## Project structure

| Directory      | Purpose                                                                 |
| -------------- | ----------------------------------------------------------------------- |
| `parser/`      | IEC 61131-3 ST parser, produces an AST                                  |
| `formatter/`   | Takes AST, applies formatting rules (indentation, spacing, line wrap)   |
| `handlers/`    | File-type handlers: `STHandler` (plain ST) and `XMLHandler` (TwinCAT XML wrapping ST in CDATA) |
| `handlers/testdata/` | Golden test inputs (`*.in.*`) and expected outputs (`*.golden.*`) |
| `internal/lexer/` | Lexer used by parser and tests                                       |

Entry point is `main.go`, which wires the handler registry, expands file paths, and dispatches to handlers.

## Golden tests

Tests in `handlers/golden_test.go` compare formatter output against `.golden.*` files. After changing formatting logic, regenerate:

```bash
task golden
```

The golden tests also assert **idempotency**: re-formatting output must not change it.

## Toolchain versions

- Go: see `go.mod` (currently 1.26.5)
- golangci-lint v2.13.2 (run via `go run`, not installed locally)
- goreleaser v2.18.1 (for releases, also via `go run`)
- Linter config: `.golangci.yml` — uses golangci-lint v2 config format (`version: "2"`)

## Linter specifics

- `gosec` is disabled in `_test.go` files and for rules G115, G304, G306, G703.
- `goimports` groups local imports under `github.com/ysmilda/stformat`.
- `revive` enforces: blank-imports, context-as-argument, dot-imports, error-return, error-strings, increment-decrement, var-naming.

## Supported file extensions

Plain ST: `.st`, `.iecst`, `.tcst`
TwinCAT XML: `.tcpou`, `.tcgvl`, `.tcdut`, `.tcio`

## Release process

GoReleaser builds cross-platform binaries (linux/darwin/windows, amd64/arm64), Docker images (ghcr.io), Homebrew casks, and winget packages. Run `task release-snapshot` for a local test build. CGO is disabled.
