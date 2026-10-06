package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
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

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "stformat: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		checkFlag   = flag.Bool("check", false, "Check whether files are formatted; exit 1 if any need formatting")
		diffFlag    = flag.Bool("diff", false, "Print diff for unformatted files")
		writeFlag   = flag.Bool("w", true, "Write formatted output to files (default true)")
		stdinFlag   = flag.Bool("stdin", false, "Read source from stdin (ST only)")
		stdoutFlag  = flag.Bool("stdout", false, "Write formatted output to stdout")
		quietFlag   = flag.Bool("q", false, "Suppress informational output")
		versionFlag = flag.Bool("version", false, "Print version and exit")
	)
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), `stformat - Structured Text formatter

Usage:
  stformat [flags] [path...]

Format .st, .iecst, and TwinCAT XML (.TcPOU, .TcGVL, .TcDUT) files.

Flags:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionFlag {
		fmt.Printf("stformat %s\ncommit: %s\nbuilt:  %s\n", version, commit, date)
		return nil
	}

	registry := handlers.NewRegistry()

	// Read from stdin. The input is fed through formatter.Copy so that a
	// "// stformat:ignore" stream is piped through without being parsed.
	if *stdinFlag {
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		var formatted bytes.Buffer
		if _, err := formatter.Copy(&formatted, bytes.NewReader(src)); err != nil {
			return err
		}
		if *checkFlag {
			if formatted.String() != string(src) {
				return fmt.Errorf("input is not formatted")
			}
			return nil
		}
		fmt.Print(formatted.String())
		return nil
	}

	paths := flag.Args()
	if len(paths) == 0 {
		flag.Usage()
		return nil
	}

	files, err := expandFiles(paths, registry.Extensions())
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no supported files found")
	}

	// Resolve the handler for each file up front (cheap and deterministic);
	// unsupported files are skipped and reported.
	type job struct {
		index int
		file  string
		h     handlers.Handler
	}
	var jobs []job
	for i, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		handler, err := registry.ForExtension(ext)
		if err != nil {
			if !*quietFlag {
				fmt.Fprintf(os.Stderr, "stformat: skipping %s: %v\n", file, err)
			}
			continue
		}
		jobs = append(jobs, job{index: i, file: file, h: handler})
	}

	// Read and format files concurrently using a fixed worker pool. Results
	// are collected by job index so they can be processed in file order,
	// keeping output deterministic.
	type result struct {
		index     int
		data      []byte
		formatted []byte
		err       error
	}

	workers := runtime.NumCPU()
	if workers > len(jobs) {
		workers = len(jobs)
	}

	ch := make(chan job)
	results := make(chan result, len(jobs))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				data, err := os.ReadFile(j.file)
				if err != nil {
					results <- result{index: j.index, err: err}
					continue
				}
				formatted, err := j.h.Format(data)
				if err != nil {
					results <- result{index: j.index, err: fmt.Errorf("%s: %w", j.file, err)}
					continue
				}
				results <- result{index: j.index, data: data, formatted: formatted}
			}
		}()
	}

	go func() {
		for _, j := range jobs {
			ch <- j
		}
		close(ch)
		wg.Wait()
		close(results)
	}()

	ordered := make([]result, len(files))
	for r := range results {
		ordered[r.index] = r
	}

	var unformatted []string
	for _, j := range jobs {
		file := j.file
		r := ordered[j.index]
		if r.err != nil {
			return r.err
		}
		data := r.data
		formatted := r.formatted

		changed := string(formatted) != string(data)
		if !changed {
			continue
		}
		if *checkFlag {
			// Report every file, not just the first, and leave the exit
			// code to the end so the list is complete.
			unformatted = append(unformatted, file)
			continue
		}
		if *diffFlag {
			fmt.Printf("%s\n", file)
			printDiff(string(data), string(formatted))
			continue
		}
		if *writeFlag {
			if err := os.WriteFile(file, formatted, 0644); err != nil {
				return err
			}
			if !*quietFlag {
				fmt.Printf("formatted: %s\n", file)
			}
		}
		if *stdoutFlag || !*writeFlag {
			fmt.Print(string(formatted))
		}
	}

	if len(unformatted) > 0 {
		for _, file := range unformatted {
			fmt.Printf("unformatted: %s\n", file)
		}
		if !*quietFlag {
			fmt.Printf("%d file(s) need formatting\n", len(unformatted))
		}
		os.Exit(1)
	}

	return nil
}

// expandFiles expands directory paths into files with a supported extension.
// A file named both directly and through a directory is kept once.
func expandFiles(paths, exts []string) ([]string, error) {
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
		if info.IsDir() {
			err := filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				if slices.Contains(exts, strings.ToLower(filepath.Ext(path))) {
					add(path)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			add(p)
		}
	}
	return files, nil
}

// printDiff prints the lines that differ between the old and the new content.
// The caller prints the file name.
func printDiff(old, new string) {
	for _, l := range diffLines(strings.Split(old, "\n"), strings.Split(new, "\n")) {
		fmt.Println(l)
	}
}
