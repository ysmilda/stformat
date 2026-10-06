# stformat

An opiniated formatter for IEC 61131-3 Structured Text (ST) and TwinCAT XML project files.

It follows the following rules:

- Keywords (`IF`, `AND`, `VAR`, ...) are uppercased; identifiers keep their original case.
- Indentation is a tab per block level.
- Lines are wrapped at 120 characters; long function calls and IF/ELSIF conditions are broken across lines. A wrapped condition puts `THEN` on a line of its own.
- Binary operators (`:=`, `+`, `<`, ...) are surrounded by spaces.
- Nothing is aligned: declarations keep one space around `:`, assignments one space around `:=`.
- Comments start with a space and a capital letter (`// Text`, `(* Text *)`). A comment at the end of a line stays there, separated by two spaces, and does not count towards the line length.
- Case labels are separated from each other by a blank line (the first label after `OF` starts on the next line directly).
- The line ending of the input is kept: a CRLF file stays CRLF, an LF file stays LF. A file that mixes both is normalised to the one it uses most.
- Formatting can be switched off with a comment directive (see below).
- Output is idempotent: formatting an already formatted file changes nothing.

## Directives

Three comment directives switch formatting off. The name is matched case insensitively and may be written `stformat:off` or `stformat : off`.

| Directive            | Effect                                                          |
| -------------------- | --------------------------------------------------------------- |
| `// stformat:ignore` | Leave the whole file untouched (top of file only, see below)    |
| `// stformat:off`    | Copy everything up to `// stformat:on` verbatim                 |
| `// stformat:on`     | Resume formatting after an `off` directive                      |

`stformat:ignore` applies to the whole file, so it only counts in the comment block at the top of the file, above the first declaration. Further down it is an ordinary comment and formats nothing. A directive may stand on its own line or trail code on a line. Text inside an ignored region is copied byte for byte: it is not re-indented or wrapped, and a string literal spanning several lines keeps its content untouched. Directives are never rewritten themselves, so their spelling and case survive formatting.

```st
// stformat:ignore          <- top of file: nothing below is formatted
PROGRAM Untouched
    x:=1;
END_PROGRAM
```

```st
// stformat:off
VAR
	ugly    :   INT;   // copied verbatim, not re-aligned
END_VAR
// stformat:on
```

### From a stream

`stformat -stdin < file.st` honours the directives like a file does. For code that embeds the formatter, `formatter.Copy(dst, src)` formats any `io.Reader` into any `io.Writer`:

```go
n, err := formatter.Copy(os.Stdout, os.Stdin)
```

A file-level `stformat:ignore` is streamed straight through without being parsed, so an ignored stream can be of any size. Every other input is read in full first, because wrapping a line at 120 characters needs the whole line and the block structure around it.

When embedding, `formatter.FormatWith(source, ending)` imposes the line ending instead of detecting it. `XMLHandler` uses it so that the line breaks it inserts around a CDATA section follow the rest of the file, which a single-line section has no ending of its own to copy.

Supported formats:

- Plain ST: `.st`, `.iecst`, `.tcst`
- TwinCAT XML: `.tcpou`, `.tcgvl`, `.tcdut`, `.tcio`, `.tcproj`, `.xml` (formats the ST inside CDATA sections)

## Install

### Package managers

| Package manager     | Command                                                        |
| ------------------- | -------------------------------------------------------------- |
| Go (source)         | `go install github.com/ysmilda/stformat@latest`                |
| Release binary      | archives + checksums on the [releases] page                    |

## Usage

```bash
stformat .                  # format files in place
stformat -check .           # exit 1 if any file needs formatting
stformat -diff file.st      # show what would change, without writing
stformat -stdin < file.st   # read ST from stdin
```

| Flag       | Default | Description                                  |
| ---------- | ------- | -------------------------------------------- |
| `-check`   | `false` | Check formatting; list every file that differs and exit 1 |
| `-diff`    | `false` | Print a diff for unformatted files           |
| `-w`       | `true`  | Write formatted output to files              |
| `-stdin`   | `false` | Read ST from stdin                           |
| `-stdout`  | `false` | Write the result to stdout                   |
| `-q`       | `false` | Suppress informational output                |
| `-version` | `false` | Print version, commit and build date         |

`-check` takes precedence over `-diff`, so `-diff -check` still exits 1.