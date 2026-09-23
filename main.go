package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

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

	// Read from stdin
	if *stdinFlag {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		h := &handlers.STHandler{}
		formatted, err := h.Format(data)
		if err != nil {
			return err
		}
		if *checkFlag {
			if string(formatted) != string(data) {
				return fmt.Errorf("input is not formatted")
			}
			return nil
		}
		fmt.Print(string(formatted))
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

	unformatted := 0
	for _, j := range jobs {
		file := j.file
		r := ordered[j.index]
		if r.err != nil {
			return r.err
		}
		data := r.data
		formatted := r.formatted

		changed := string(formatted) != string(data)
		if changed {
			unformatted++
			if *checkFlag {
				fmt.Printf("%s\n", file)
				if *diffFlag {
					printDiff(file, string(data), string(formatted))
				}
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
			if *stdoutFlag {
				fmt.Print(string(formatted))
			}
		}
		if !*checkFlag && !*writeFlag && !*stdoutFlag && changed {
			// No write requested; print formatted output
			fmt.Print(string(formatted))
		}
	}

	if *checkFlag && unformatted > 0 {
		return fmt.Errorf("%d file(s) need formatting", unformatted)
	}

	return nil
}

// expandFiles expands directory paths into files with a supported extension.
func expandFiles(paths, exts []string) ([]string, error) {
	var files []string
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
					files = append(files, path)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			files = append(files, p)
		}
	}
	return files, nil
}

// printDiff prints a simple unified diff between old and new content.
func printDiff(file, old, new string) {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	max := len(oldLines)
	if len(newLines) > max {
		max = len(newLines)
	}

	for i := range max {
		var o, n string
		if i < len(oldLines) {
			o = oldLines[i]
		}
		if i < len(newLines) {
			n = newLines[i]
		}
		if o == n {
			continue
		}
		fmt.Printf("  %s\n", file)
		fmt.Printf("  - %s\n", o)
		fmt.Printf("  + %s\n", n)
	}
}
