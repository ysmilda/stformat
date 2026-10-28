package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/ysmilda/stformat/formatter"
	"github.com/ysmilda/stformat/handlers"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// errUnformatted is returned by run when -check finds input that is not
// formatted. The offending files have already been listed, so main only turns
// it into the exit code.
var errUnformatted = errors.New("input is not formatted")

// Exit codes. A script can tell input that needs formatting apart from a run
// that failed: only exitUnformatted means every file was read and formatted.
const (
	exitUnformatted = 1 // -check found input that is not formatted
	exitError       = 2 // a file could not be read, parsed or formatted
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, errUnformatted) {
			fmt.Fprintf(os.Stderr, "stformat: %v\n", err)
		}
		os.Exit(exitCode(err))
	}
}

// exitCode returns the exit code for an error returned by run.
func exitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errUnformatted):
		return exitUnformatted
	default:
		return exitError
	}
}

// options holds the parsed command line.
type options struct {
	check, diff, write, stdin, stdout, quiet bool
	// exclude holds the name patterns of files and directories to skip
	// when walking a directory.
	exclude []string
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	var opts options
	fset := flag.NewFlagSet("stformat", flag.ContinueOnError)
	fset.SetOutput(stderr)
	fset.BoolVar(&opts.check, "check", false, "List files that are not formatted and exit 1 if there are any")
	fset.BoolVar(&opts.diff, "diff", false, "Print a diff for files that are not formatted instead of writing them")
	fset.BoolVar(&opts.write, "w", true, "Write formatted output back to the files")
	fset.BoolVar(&opts.stdin, "stdin", false, "Read source from stdin (ST only) and write the result to stdout")
	fset.BoolVar(&opts.stdout, "stdout", false, "Write formatted output to stdout instead of to the files")
	fset.BoolVar(&opts.quiet, "q", false, "Suppress informational output")
	fset.Func("exclude", "Skip files and directories whose name matches this pattern while walking a directory (repeatable, e.g. -exclude '*_old.st')", func(pattern string) error {
		if _, err := filepath.Match(pattern, ""); err != nil {
			return fmt.Errorf("bad pattern %q: %w", pattern, err)
		}
		opts.exclude = append(opts.exclude, pattern)
		return nil
	})
	versionFlag := fset.Bool("version", false, "Print version and exit")
	fset.Usage = func() {
		_, _ = fmt.Fprintf(fset.Output(), `stformat - Structured Text formatter

Usage:
  stformat [flags] [path...]

Formats plain ST (.st, .iecst, .tcst) and the ST inside TwinCAT XML
(.TcPOU, .TcGVL, .TcDUT, .TcIO). Directories are searched recursively.

Flags:
`)
		fset.PrintDefaults()
	}
	if err := fset.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if *versionFlag {
		_, err := fmt.Fprintf(stdout, "stformat %s\ncommit: %s\nbuilt:  %s\n", version, commit, date)
		return err
	}

	if opts.stdin {
		return runStdin(opts, stdin, stdout)
	}

	paths := fset.Args()
	if len(paths) == 0 {
		fset.Usage()
		return nil
	}
	return runFiles(opts, paths, stdout, stderr)
}

// runStdin formats standard input. The input is fed through formatter.Copy so
// that a "// stformat:ignore" stream is piped through without being parsed.
func runStdin(opts options, stdin io.Reader, stdout io.Writer) error {
	src, err := io.ReadAll(stdin)
	if err != nil {
		return err
	}
	var formatted bytes.Buffer
	if _, err := formatter.Copy(&formatted, bytes.NewReader(src)); err != nil {
		return fmt.Errorf("<stdin>: %w", err)
	}
	if !opts.check && !opts.diff {
		_, err := stdout.Write(formatted.Bytes())
		return err
	}
	if bytes.Equal(formatted.Bytes(), src) {
		return nil
	}
	if opts.diff {
		printDiff(stdout, "<stdin>", string(src), formatted.String())
	}
	if opts.check {
		if _, err := fmt.Fprintln(stdout, "unformatted: <stdin>"); err != nil {
			return err
		}
		return errUnformatted
	}
	return nil
}

// result is the outcome of formatting one file.
type result struct {
	file      string
	skipped   bool // no handler for the file
	data      []byte
	formatted []byte
	err       error
}

// runFiles formats the files named by paths. Every file is formatted before
// any is written, so a file that fails to format leaves all of them untouched.
func runFiles(opts options, paths []string, stdout, stderr io.Writer) error {
	registry := handlers.NewRegistry()
	files, err := expandFiles(paths, registry.Extensions(), opts.exclude)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no supported files found")
	}

	results := formatFiles(registry, files, opts.quiet, stderr)

	var errs []error
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	var unformatted int
	for _, r := range results {
		if r.skipped {
			continue
		}
		changed := !bytes.Equal(r.formatted, r.data)
		switch {
		case opts.stdout || (!opts.write && !opts.check && !opts.diff):
			// Every file is printed, formatted or not, so the output is the
			// whole formatted input.
			if _, err := stdout.Write(r.formatted); err != nil {
				return err
			}
		case !changed:
		case opts.check || opts.diff:
			unformatted++
			if opts.diff {
				printDiff(stdout, r.file, string(r.data), string(r.formatted))
			}
			if opts.check {
				if _, err := fmt.Fprintf(stdout, "unformatted: %s\n", r.file); err != nil {
					return err
				}
			}
		default:
			// The mode only applies to a new file; this one keeps its own.
			if err := os.WriteFile(r.file, r.formatted, 0o644); err != nil {
				return err
			}
			if !opts.quiet {
				if _, err := fmt.Fprintf(stdout, "formatted: %s\n", r.file); err != nil {
					return err
				}
			}
		}
	}

	if opts.check && unformatted > 0 {
		if !opts.quiet {
			if _, err := fmt.Fprintf(stdout, "%d file(s) need formatting\n", unformatted); err != nil {
				return err
			}
		}
		return errUnformatted
	}
	return nil
}

// formatFiles reads and formats files concurrently and returns the results in
// file order. A file without a handler is reported on stderr and marked as
// skipped.
func formatFiles(registry *handlers.Registry, files []string, quiet bool, stderr io.Writer) []result {
	results := make([]result, len(files))
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for i, file := range files {
		h, err := registry.ForExtension(strings.ToLower(filepath.Ext(file)))
		if err != nil {
			if !quiet {
				_, _ = fmt.Fprintf(stderr, "stformat: skipping %s: %v\n", file, err)
			}
			results[i] = result{file: file, skipped: true}
			continue
		}
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = formatFile(h, file)
		})
	}
	wg.Wait()
	return results
}

// formatFile reads one file and formats it with h.
func formatFile(h handlers.Handler, file string) result {
	data, err := os.ReadFile(file)
	if err != nil {
		return result{file: file, err: err}
	}
	formatted, err := h.Format(data)
	if err != nil {
		return result{file: file, err: fmt.Errorf("%s: %w", file, err)}
	}
	return result{file: file, data: data, formatted: formatted}
}

// expandFiles expands directory paths into files with a supported extension.
// A file named both directly and through a directory is kept once. While
// walking, it skips what skipped reports; a path named on the command line is
// always used.
func expandFiles(paths, exts, exclude []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)
	add := func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		files = append(files, path)
	}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			add(p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path != p && skipped(d, exclude) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() && slices.Contains(exts, strings.ToLower(filepath.Ext(path))) {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// printDiff prints the changes formatting makes to a file as a unified diff.
func printDiff(w io.Writer, name, old, new string) {
	_, _ = io.WriteString(w, unifiedDiff(name, old, new))
}

// generatedDirs are directories that hold build output or copies of code
// rather than sources: TwinCAT's generated folders and the usual .NET ones.
var generatedDirs = []string{"_Boot", "_CompileInfo", "_Deployment", "_Libraries", "bin", "obj"}

// skipped reports whether a directory walk leaves d out: a hidden entry (its
// name starts with '.', such as .git), a generated directory, or an entry
// whose name matches one of the exclude patterns.
func skipped(d fs.DirEntry, exclude []string) bool {
	name := d.Name()
	if strings.HasPrefix(name, ".") {
		return true
	}
	if d.IsDir() && slices.ContainsFunc(generatedDirs, func(g string) bool { return strings.EqualFold(g, name) }) {
		return true
	}
	return slices.ContainsFunc(exclude, func(pattern string) bool {
		ok, _ := filepath.Match(pattern, name)
		return ok
	})
}
