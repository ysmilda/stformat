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
| `extension.yml` | paths: `extension/**`, `**/*.go`, `go.mod`, `go.sum` | builds both VSIXs (`format`, `format15`) on windows-latest |
| `release.yml` | `v*` tags                                            | goreleaser release + both VSIXs                           |

`ci.yml` deliberately has no `paths` filter. A path-filtered workflow that is a
required status check never reports on PRs that do not touch those files, and
GitHub blocks the merge on a check that is still "Expected". Only add path
filters to workflows whose jobs are not required.

`extension.yml` watches the Go sources as well as `extension/**`, because
both VSIX projects build `extension/format/Resources/stformat.exe` from them in
a `BeforeTargets="PrepareForBuild"` step. A Go change can therefore break the
VSIX build. To build the extension locally, run the msbuild commands from
`extension.yml` against `extension\format\format.csproj` and
`extension\format15\format15.csproj`.

Every workflow takes its Go version from `go.mod` (`go-version-file`), so
bumping the `go` line in `go.mod` is the only change needed.

To run a single test:

```bash
go test ./handlers -run TestSTFiles/addresses
```

## Project lossless test

`TestProjectFormatLossless` in `handlers/project_test.go` formats every
supported file in an ST/TwinCAT project tree and independently verifies
(without using the internal lexer) that only whitespace and character case
changed. It runs on the sample project in `handlers/benchdata` by default;
point it at a real project with `STFORMAT_PROJECT_DIR`:

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
| `extension/format/` | Visual Studio extension for VS 2022+ and the 64-bit TwinCAT XAE Shell (4026) |
| `extension/format15/` | The same sources (linked from `format/`) built for VS 2017/2019 and the 32-bit XAE Shell (4024); keep its manifest `Identity` equal to `format/` |

Entry point is `main.go`: `run(args, stdin, stdout, stderr)` parses flags, expands file paths, formats every file concurrently and only then writes; `cli_test.go` drives it directly. Exit codes: 1 means `-check` found unformatted files, 2 means an error (`exitCode`). Directory walks skip hidden and generated directories and `-exclude` patterns (`skipped`). `diff.go` writes `-diff` as a unified diff.

Formatting is a single token pass (`formatter.go`, no AST) that writes each statement on one line and records which token each piece of the line is. When a line ends, `layout.go` lays it out from those tokens: bracketed lists and operator chains break when the line is over 120 columns (tab = 4) or when the source broke them at one of their break points. The rules, and the policy for user line breaks, are in the comment at the top of `layout.go`. No pass re-reads the output text, so strings, comments, pragmas and ignored regions never need special-casing.

The token pass also tracks the block and bracket stack: `Format`/`FormatWith` return a `*SyntaxError` (with a line) for structure that does not close, except a POU left open (TwinCAT stores declarations without `END_`) and files with `stformat:off` regions. `NAMESPACE`, `CONFIGURATION`, `RESOURCE`, `STEP`, `TRANSITION`, `GET` and `SET` stay identifiers in the lexer; `findOpeners` makes them block openers only at a statement start with their `END_` keyword further on.

## Golden tests

Tests in `handlers/golden_test.go` compare formatter output against `.golden.*` files. After changing formatting logic, regenerate:

```bash
task golden
```

The golden tests also assert **idempotency**: re-formatting output must not change it,
and **losslessness** through `formatter.Verify`: the output must lex to the same
tokens, with only whitespace, keyword case and comment spacing changed.

## Safety net and fuzzing

`formatter.Format` checks its own output before returning it, with the rules of
`formatter.Verify(before, after)`: if formatting would add, drop or change a
token, the file fails with an error and is not written. Do not weaken the check
to make a test pass; fix the formatter.

`FuzzFormat` in `formatter/fuzz_test.go` holds the layout to `Verify` and
idempotency on arbitrary input, seeded from the golden inputs. Fuzz input is
rarely well formed, so it uses `layout()`, which skips the structure check;
layout tests on deliberately malformed input use it too. Run the fuzzer after any
change to the lexer or the layout:

```bash
go test ./formatter -run '^$' -fuzz FuzzFormat -fuzztime 5m
```

Failing inputs it finds land in `formatter/testdata/fuzz/FuzzFormat/` and are
replayed by every plain `go test` run; commit them with the fix.

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
