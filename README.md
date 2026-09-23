# stformat

An opiniated formatter for IEC 61131-3 Structured Text (ST) and TwinCAT XML project files.

It follows the following rules:

- Keywords (`IF`, `AND`, `VAR`, ...) are uppercased; identifiers keep their original case.
- Indentation is a tab per block level.
- Lines are wrapped at 120 characters; long function calls and IF/ELSIF conditions are broken across lines.
- Binary operators (`:=`, `+`, `<`, ...) are surrounded by spaces.
- `VAR` declarations are aligned on `:`, assignments on `:=`.
- Comments start with a space and a capital letter (`// Text`, `(* Text *)`).
- Case labels are separated from each other by a blank line (the first label after `OF` starts on the next line directly).
- Output is idempotent: formatting an already formatted file changes nothing.

Supported formats:

- Plain ST: `.st`, `.iecst`, `.tcst`
- TwinCAT XML: `.tcpou`, `.tcgvl`, `.tcdut`, `.tcio` (formats the ST inside CDATA sections)

## Install

### Package managers

| Package manager     | Command                                                        |
| ------------------- | -------------------------------------------------------------- |
| Homebrew (macOS)    | `brew install ysmilda/tap/stformat`                            |
| Windows (winget)    | `winget install --id Ysmilda.stformat -e`                      |
| Docker              | `docker run --rm ghcr.io/ysmilda/stformat:latest -version` |
| Go (source)         | `go install github.com/ysmilda/stformat@latest`                |
| Release binary      | archives + checksums on the [releases] page                    |

> Homebrew and winget publish on each [tagged release]; Docker images are
> multi-platform (`linux/amd64`, `linux/arm64`) and live at
> `ghcr.io/ysmilda/stformat`.

## Usage

```bash
stformat .                  # format files in place
stformat -check .           # exit 1 if any file needs formatting
stformat -diff file.st      # show what would change, without writing
stformat -stdin < file.st   # read ST from stdin
```

| Flag       | Default | Description                                  |
| ---------- | ------- | -------------------------------------------- |
| `-check`   | `false` | Check formatting; exit 1 if any file differs |
| `-diff`    | `false` | Print a diff for unformatted files           |
| `-w`       | `true`  | Write formatted output to files              |
| `-stdin`   | `false` | Read ST from stdin                           |
| `-stdout`  | `false` | Write the result to stdout                   |
| `-q`       | `false` | Suppress informational output                |
| `-version` | `false` | Print version, commit and build date         |