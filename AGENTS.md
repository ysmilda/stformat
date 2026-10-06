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
task coverage     # test with coverage, print the per-function report
```

## CI

Three workflows in `.github/workflows/`:

| Workflow      | Trigger                                              | Contents                                                 |
| ------------- | ---------------------------------------------------- | -------------------------------------------------------- |
| `ci.yml`      | every push/PR, no path filter                        | build, vet, `go mod tidy -diff`, tests + coverage, lint   |
| `extension.yml` | paths: `extension/**`, `**/*.go`, `go.mod`, `go.sum` | VSIX build on windows-latest, uploads `stformat-ci.vsix`  |
| `release.yml` | `v*` tags                                            | goreleaser release + VSIX                                 |

`ci.yml` deliberately has no `paths` filter. A path-filtered workflow that is a
required status check never reports on PRs that do not touch those files, and
GitHub blocks the merge on a check that is still "Expected". Only add path
filters to workflows whose jobs are not required.

`extension.yml` watches the Go sources as well as `extension/**`, because
`format.csproj` builds `Resources/stformat.exe` from them in a
`BeforeTargets="PrepareForBuild"` step. A Go change can therefore break the VSIX
build. To build the extension locally, run the msbuild commands from
`extension.yml` against `extension\format\format.csproj`.

Each workflow pins its Go version in a workflow-level `env: GO_VERSION`. Bump
the value in every workflow file that needs it.

To run a single test:

```bash
go test ./handlers -run TestSTFiles/addresses
```

## Real-project lossless test (opt-in)

`TestProjectFormatLossless` in `handlers/project_test.go` formats every
supported file in a real ST/TwinCAT project tree and independently verifies
(without using the internal lexer) that only whitespace and character case
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
| `formatter/`   | Formats the token stream: indentation, spacing, line wrap, comment and directive handling |
| `handlers/`    | File-type handlers: `STHandler` (plain ST) and `XMLHandler` (TwinCAT XML wrapping ST in CDATA) |
| `handlers/testdata/` | Golden test inputs (`*.in.*`) and expected outputs (`*.golden.*`) |
| `internal/lexer/` | Tokenizer shared by the formatter and tests                          |

Entry point is `main.go`, which wires the handler registry, expands file paths, and dispatches to handlers. Formatting is token-driven end to end: there is no AST and no separate parser pass.

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
