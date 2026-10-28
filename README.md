# stformat

> [!NOTE]
> This project is AI generated but reviewed and maintained by a human.

An opiniated formatter for IEC 61131-3 Structured Text (ST) and TwinCAT XML project files.

It follows the following rules:

- Keywords (`IF`, `AND`, `VAR`, ...) are uppercased; identifiers keep their original case.
- Indentation is a tab per block level.
- Lines are wrapped at 120 columns (a tab counts as four). A line that is too long breaks open its last bracketed list (call arguments, array or enum values), one item per line; if that does not help, its expression breaks before each operator of the lowest precedence, one operand per line. Lines produced this way are wrapped again by the same rules. An IF/ELSIF/WHILE condition joined by `AND`/`OR`/`XOR` is laid out one operand per line, with `THEN`/`DO` on a line of its own.
- An enum's value list is always broken open, one value per line, and so is any bracketed list that holds a comment: the bracket ends its line, each item gets a line of its own and the closing bracket goes on its own line.
- Line breaks you write are kept where a break is allowed: a break after `(` or `[`, around a comma or before the closing bracket keeps that list broken open, and a break before or after an operator keeps that chain broken. Any other line break inside a statement is removed.
- One blank line you write between two lines is kept, to separate parts of a block; several in a row become one. A blank line right after a block opener or an opening bracket, or right before a block closer or a closing bracket, is removed.
- A closed block (`END_IF`, `END_VAR`, ...) is followed by a blank line, unless another closer, `ELSE`, `ELSIF` or `UNTIL` comes next. A `;` written after the closer (`END_IF;`) stays on its line.
- Binary operators (`:=`, `+`, `<`, ...) are surrounded by spaces; a sign sticks to its operand (`-1`, `a * -b`).
- Nothing is aligned: declarations keep one space around `:`, assignments one space around `:=`.
- A space is put between a comment opener and a word glued to it (`//note` becomes `// note`). A line comment loses its trailing blanks. Nothing else in a comment changes: banners, `///` doc comments and commented-out code keep their exact text. A comment at the end of a line stays there, separated by one space, and does not count towards the line length.
- Case labels are separated from each other by a blank line (the first label after `OF` starts on the next line directly).
- The line ending of the input is kept: a CRLF file stays CRLF, an LF file stays LF. A file that mixes both is normalised to the one it uses most.
- Formatting can be switched off with a comment directive (see below).
- Output is idempotent: formatting an already formatted file changes nothing.
- Formatting never changes the code itself. Before any output is used, it is checked to hold the same tokens as the input (only whitespace, keyword case and comment spacing may differ); a file that fails the check is reported and left untouched. Characters that are not ST are kept as they are, and a UTF-8 byte order mark is preserved.
- A file that is not well formed is reported with its line and left untouched: a block that is opened and never closed (`IF` without `END_IF`), closed without being opened, or closed while a block inside it is still open, and brackets that do not match. A POU may stay open, because TwinCAT stores a POU's declaration without its `END_` keyword. Files with `stformat:off` regions are not checked.
- Headers are kept on one line: modifiers, return types (`POINTER TO INT`, `ARRAY[0..1] OF INT`, `STRING(80)`) and `EXTENDS`/`IMPLEMENTS` lists.
- `NAMESPACE`, `CONFIGURATION`/`RESOURCE`, property `GET`/`SET` accessors and SFC `STEP`/`INITIAL_STEP`/`TRANSITION` blocks are laid out as blocks. Their openers are common names in PLC code (a `Step` counter, a `Set` method), so they only count as openers where their `END_` keyword closes them; anywhere else they stay names.

## Directives

Three comment directives switch formatting off. Ignored sections do not nest: the first `on` after an `off` ends the section. The name is matched case insensitively and may be written `stformat:off` or `stformat : off`.

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
- TwinCAT XML: `.tcpou`, `.tcgvl`, `.tcdut`, `.tcio` (formats the ST inside the CDATA sections of `<Declaration>` and `<ST>` elements; a section that does not look like ST is left alone)

## Install

### Package managers

| Package manager     | Command                                                        |
| ------------------- | -------------------------------------------------------------- |
| Go (source)         | `go install github.com/ysmilda/stformat@latest`                |
| Release binary      | archives + checksums on the [releases] page                    |

### Visual Studio and TwinCAT XAE

Each release carries two builds of the extension:

| File                             | For                                                          |
| -------------------------------- | ------------------------------------------------------------ |
| `stformat-<version>.vsix`        | Visual Studio 2022 and later, 64-bit XAE Shell (TwinCAT 4026) |
| `stformat-<version>-vs2017.vsix` | Visual Studio 2017/2019, 32-bit XAE Shell (TwinCAT 4024)      |

Open the `.vsix` to install it into Visual Studio. The VSIX installer does not
recognise the XAE Shell, so extract the matching file into the shell's
extensions folder from an elevated PowerShell and restart the shell:

```powershell
$shell = 'C:\Program Files (x86)\Beckhoff\TcXaeShell'   # 4024; 'C:\Program Files\Beckhoff\TcXaeShell' for 4026
$vsix  = 'stformat-<version>-vs2017.vsix'               # stformat-<version>.vsix for 4026
$ext   = "$shell\Common7\IDE\Extensions"
Add-Type -AssemblyName System.IO.Compression.FileSystem
if (Test-Path "$ext\StFormat") { Remove-Item "$ext\StFormat" -Recurse -Force }
[IO.Compression.ZipFile]::ExtractToDirectory((Resolve-Path $vsix), "$ext\StFormat")
(Get-Item "$ext\extensions.configurationchanged").LastWriteTime = Get-Date
```

The extension adds:

- **Format Structured Text** (Tools menu): formats the active document.
- Format on save, which can be turned off under **Tools > Options > StFormat**.

TwinCAT POUs, GVLs, DUTs and interfaces are formatted through the TwinCAT
automation interface, so the open editor is updated in place without a
"modified outside the editor" prompt. Other files are formatted in their text
editor buffer.

Undo (Ctrl+Z) reverts formatting in one step in Visual Studio text editors,
such as plain `.st` files. It does **not** undo formatting in TwinCAT editors:
TwinCAT does not record changes made through its automation interface in its
undo history, and undoing there can leave the editor showing text that no
longer matches the object. Use source control to revert formatting.

Problems are logged to `%LOCALAPPDATA%\StFormat\stformat.log`.

## Usage

```bash
stformat .                  # format files in place
stformat -check .           # exit 1 if any file needs formatting
stformat -diff file.st      # show what would change as a unified diff, without writing
stformat -stdin < file.st   # read ST from stdin
```

| Flag       | Default | Description                                  |
| ---------- | ------- | -------------------------------------------- |
| `-check`   | `false` | List every file that is not formatted and exit 1; combine with `-diff` to see the changes |
| `-diff`    | `false` | Print a unified diff for unformatted files instead of writing them; `git apply` and `patch -p1` read it |
| `-exclude` |         | Skip files and directories whose name matches this pattern while walking a directory; repeatable (`-exclude '*_old.st' -exclude Generated`) |
| `-w`       | `true`  | Write formatted output back to the files; `-w=false` prints every file instead |
| `-stdin`   | `false` | Read ST from stdin and write the result to stdout |
| `-stdout`  | `false` | Print every file, formatted or not, instead of writing it |
| `-q`       | `false` | Suppress informational output                |
| `-version` | `false` | Print version, commit and build date         |

All files are formatted before any is written: if one file cannot be formatted (for example malformed XML, or an `IF` without `END_IF`), nothing is written and the error names the file and line.

When walking a directory, hidden entries (`.git`, ...) and generated directories (`_Boot`, `_CompileInfo`, `_Deployment`, `_Libraries`, `bin`, `obj`) are skipped. A path named on the command line is always formatted.

| Exit code | Meaning |
| --------- | ------- |
| `0`       | Success; with `-check`, every file is formatted |
| `1`       | `-check` found files that need formatting |
| `2`       | An error: a file could not be read, parsed or formatted, or a flag was wrong |
