package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	formattedSrc   = "x := 1;\n"
	unformattedSrc = "x:=1;\n"
)

// writeFiles creates the named files in a fresh directory and returns their
// paths in the given order.
func writeFiles(t *testing.T, files ...string) []string {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for i := 0; i < len(files); i += 2 {
		p := filepath.Join(dir, files[i])
		if err := os.WriteFile(p, []byte(files[i+1]), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return paths
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// runCLI runs the command line with args and returns what it printed.
func runCLI(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := run(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), err
}

func TestStdoutPrintsEveryFile(t *testing.T) {
	t.Parallel()
	paths := writeFiles(t, "a.st", formattedSrc, "b.st", unformattedSrc)
	out, err := runCLI(t, "", append([]string{"-stdout"}, paths...)...)
	if err != nil {
		t.Fatal(err)
	}
	if want := formattedSrc + formattedSrc; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
	if got := readFile(t, paths[1]); got != unformattedSrc {
		t.Errorf("-stdout wrote the file: %q", got)
	}
}

func TestNoWritePrintsEveryFile(t *testing.T) {
	t.Parallel()
	paths := writeFiles(t, "a.st", formattedSrc)
	out, err := runCLI(t, "", "-w=false", paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if out != formattedSrc {
		t.Errorf("output = %q, want %q", out, formattedSrc)
	}
}

func TestCheckWithDiff(t *testing.T) {
	t.Parallel()
	paths := writeFiles(t, "a.st", formattedSrc, "b.st", unformattedSrc)
	out, err := runCLI(t, "", append([]string{"-check", "-diff", "-q"}, paths...)...)
	if !errors.Is(err, errUnformatted) {
		t.Fatalf("err = %v, want errUnformatted", err)
	}
	for _, want := range []string{"-x:=1;\n", "+x := 1;\n", "unformatted: " + paths[1]} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}
	if strings.Contains(out, paths[0]) {
		t.Errorf("output %q mentions the formatted file", out)
	}
	if got := readFile(t, paths[1]); got != unformattedSrc {
		t.Errorf("-check wrote the file: %q", got)
	}
}

func TestDiffDoesNotWrite(t *testing.T) {
	t.Parallel()
	paths := writeFiles(t, "a.st", unformattedSrc)
	if _, err := runCLI(t, "", "-diff", paths[0]); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, paths[0]); got != unformattedSrc {
		t.Errorf("-diff wrote the file: %q", got)
	}
}

func TestWrite(t *testing.T) {
	t.Parallel()
	paths := writeFiles(t, "a.st", unformattedSrc)
	out, err := runCLI(t, "", paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, paths[0]); got != formattedSrc {
		t.Errorf("file = %q, want %q", got, formattedSrc)
	}
	if want := "formatted: " + paths[0] + "\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

// TestFailureWritesNothing checks that one file that cannot be formatted keeps
// every other file from being written, whatever order they come in.
func TestFailureWritesNothing(t *testing.T) {
	t.Parallel()
	paths := writeFiles(t, "a.st", unformattedSrc, "b.TcPOU", "<TcPlcObject><POU>")
	if _, err := runCLI(t, "", paths...); err == nil || !strings.Contains(err.Error(), "b.TcPOU") {
		t.Fatalf("err = %v, want a parse error naming b.TcPOU", err)
	}
	if got := readFile(t, paths[0]); got != unformattedSrc {
		t.Errorf("a.st was written although another file failed: %q", got)
	}
}

func TestStdin(t *testing.T) {
	t.Parallel()
	out, err := runCLI(t, "x:=1;", "-stdin")
	if err != nil {
		t.Fatal(err)
	}
	if out != formattedSrc {
		t.Errorf("output = %q, want %q", out, formattedSrc)
	}
	if _, err := runCLI(t, "x:=1;", "-stdin", "-check"); !errors.Is(err, errUnformatted) {
		t.Errorf("-stdin -check err = %v, want errUnformatted", err)
	}
	if _, err := runCLI(t, formattedSrc, "-stdin", "-check"); err != nil {
		t.Errorf("-stdin -check on formatted input: %v", err)
	}
}

func TestWriteKeepsMode(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no permission bits to keep")
	}
	paths := writeFiles(t, "a.st", unformattedSrc)
	if err := os.Chmod(paths[0], 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "", "-q", paths[0]); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}

// TestExitCode checks that needing formatting and failing exit differently,
// so a CI script can tell a style problem from a broken run.
func TestExitCode(t *testing.T) {
	t.Parallel()
	paths := writeFiles(t, "a.st", unformattedSrc, "bad.TcPOU", "<TcPlcObject><POU>")
	_, err := runCLI(t, "", "-check", paths[0])
	if got := exitCode(err); got != exitUnformatted {
		t.Errorf("-check on an unformatted file: exit %d, want %d", got, exitUnformatted)
	}
	_, err = runCLI(t, "", "-check", paths[1])
	if got := exitCode(err); got != exitError {
		t.Errorf("-check on a broken file: exit %d, want %d", got, exitError)
	}
	_, err = runCLI(t, "", "-check", filepath.Join(filepath.Dir(paths[0]), "missing.st"))
	if got := exitCode(err); got != exitError {
		t.Errorf("-check on a missing file: exit %d, want %d", got, exitError)
	}
	if got := exitCode(nil); got != 0 {
		t.Errorf("success: exit %d, want 0", got)
	}
}

// TestWalkSkips checks which files a directory walk picks up: hidden and
// generated directories and excluded names are skipped, but a path named on
// the command line is used whatever its name.
func TestWalkSkips(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, rel := range []string{
		"src/a.st",
		"src/old_b.st",
		".git/c.st",
		"src/_Boot/d.st",
		"src/obj/e.st",
		"src/plc/f.TcPOU",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(formattedSrc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rel := func(files []string) []string {
		out := make([]string, 0, len(files))
		for _, f := range files {
			r, err := filepath.Rel(dir, f)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, filepath.ToSlash(r))
		}
		return out
	}
	exts := []string{".st", ".tcpou"}

	files, err := expandFiles([]string{dir}, exts, []string{"old_*"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(rel(files), " "), "src/a.st src/plc/f.TcPOU"; got != want {
		t.Errorf("walk found %q, want %q", got, want)
	}

	// Named explicitly, a hidden directory and an excluded file are used.
	files, err = expandFiles([]string{filepath.Join(dir, ".git"), filepath.Join(dir, "src", "old_b.st")}, exts, []string{"old_*"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(rel(files), " "), ".git/c.st src/old_b.st"; got != want {
		t.Errorf("explicit paths gave %q, want %q", got, want)
	}

	if _, err := runCLI(t, "", "-exclude", "[", dir); err == nil {
		t.Error("a malformed -exclude pattern was accepted")
	}
}
