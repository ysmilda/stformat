package handlers

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The golden testdata has one rule per file, which makes it the right input for
// the golden tests and the wrong input for a benchmark: half its lines are
// short and half are long lines built to trip the wrap threshold. The
// benchmarks below therefore run over two corpora.
//
//   - benchdata holds ST and TwinCAT files written to look like ordinary PLC
//     code. Read throughput off these.
//   - testdata holds the golden inputs. Kept as the only corpus that drives the
//     wrapping path hard, so a regression there is still caught.
//
// Neither is a substitute for a real project; see TestProjectFormatLossless for
// the end-to-end check against one.

// corpusFile is one input file of a benchmark corpus.
type corpusFile struct {
	name string
	data []byte
}

// loadCorpus reads every file in dir with one of the given extensions, in path
// order so a benchmark is reproducible.
func loadCorpus(tb testing.TB, dir string, exts ...string) []corpusFile {
	tb.Helper()
	var names []string
	for _, ext := range exts {
		matches, err := filepath.Glob(filepath.Join(dir, "*."+ext))
		if err != nil {
			tb.Fatal(err)
		}
		names = append(names, matches...)
	}
	if len(names) == 0 {
		tb.Fatalf("no corpus files in %s for %v", dir, exts)
	}
	slices.Sort(names)

	files := make([]corpusFile, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			tb.Fatal(err)
		}
		files = append(files, corpusFile{name: filepath.Base(name), data: data})
	}
	return files
}

// worklist repeats a corpus until it holds at least target bytes. The
// benchmarks format the files one at a time rather than as one big blob,
// because that is what the CLI does: a project is many files, each read,
// formatted and written on its own, and the per-file setup cost is part of
// what a user waits for. Concatenating would also make the XML corpus one
// giant document with hundreds of root elements, which no project has.
func worklist(files []corpusFile, target int) [][]byte {
	list := make([][]byte, 0, len(files)*8)
	total := 0
	for total < target {
		for _, f := range files {
			list = append(list, f.data)
			total += len(f.data)
		}
	}
	return list
}

func worklistBytes(list [][]byte) int {
	n := 0
	for _, d := range list {
		n += len(d)
	}
	return n
}

// targetBytes is the working set every benchmark formats: large enough that an
// iteration is well above timer noise, small enough to stay cache friendly.
const targetBytes = 384 << 10

// BenchmarkCorpusST formats the benchdata ST files one by one. This is the
// headline number for plain ST.
func BenchmarkCorpusST(b *testing.B) {
	benchFormat(b, loadCorpus(b, "benchdata", "st", "iecst", "tcst"), &STHandler{})
}

// BenchmarkCorpusXML formats the benchdata TwinCAT files one by one.
func BenchmarkCorpusXML(b *testing.B) {
	benchFormat(b, loadCorpus(b, "benchdata", "tcpou", "tcgvl", "tcdut", "tcio"), &XMLHandler{})
}

// BenchmarkCorpusCRLF formats the benchdata ST files with CRLF endings, which
// take the line-ending conversion on both the input and the output.
func BenchmarkCorpusCRLF(b *testing.B) {
	files := loadCorpus(b, "benchdata", "st", "iecst", "tcst")
	for _, f := range files {
		f.data = []byte(strings.ReplaceAll(string(f.data), "\n", "\r\n"))
	}
	benchFormat(b, files, &STHandler{})
}

// BenchmarkCorpusAlreadyFormatted formats its own output, the steady state of a
// -check run over an already formatted project.
func BenchmarkCorpusAlreadyFormatted(b *testing.B) {
	h := &STHandler{}
	files := loadCorpus(b, "benchdata", "st", "iecst", "tcst")
	for i, f := range files {
		out, err := h.Format(f.data)
		if err != nil {
			b.Fatal(err)
		}
		files[i].data = out
	}
	benchFormat(b, files, h)
}

// BenchmarkWrapPath formats the golden testdata, which is mostly long lines
// built to trip the 120 column wrap. It is a regression guard for the reflow
// path, not a measurement of real projects.
func BenchmarkWrapPath(b *testing.B) {
	benchFormat(b, loadCorpus(b, "testdata", "st", "iecst", "tcst"), &STHandler{})
}

// BenchmarkWrapPathXML is the TwinCAT half of the above.
func BenchmarkWrapPathXML(b *testing.B) {
	benchFormat(b, loadCorpus(b, "testdata", "tcpou", "tcgvl", "tcdut", "tcio"), &XMLHandler{})
}

func benchFormat(b *testing.B, files []corpusFile, h Handler) {
	list := worklist(files, targetBytes)
	b.SetBytes(int64(worklistBytes(list)))
	b.ReportAllocs()
	b.ResetTimer()

	var sink int
	for b.Loop() {
		for _, data := range list {
			out, err := h.Format(data)
			if err != nil {
				b.Fatal(err)
			}
			sink += len(out)
		}
	}
	b.StopTimer()
	if sink == 0 {
		b.Fatal("no output produced")
	}
}

// TestCorpusIdempotent formats every benchdata file twice and requires the
// second pass to change nothing. These files are ordinary PLC code rather than
// rule-specific fixtures, so this is the broadest check that formatting is
// stable on realistic input.
func TestCorpusIdempotent(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	corpora := map[string][]corpusFile{
		"st":  loadCorpus(t, "benchdata", "st", "iecst", "tcst"),
		"xml": loadCorpus(t, "benchdata", "tcpou", "tcgvl", "tcdut", "tcio"),
	}
	for kind, files := range corpora {
		for _, f := range files {
			t.Run(kind+"/"+f.name, func(t *testing.T) {
				t.Parallel()
				h, err := registry.ForExtension(strings.ToLower(filepath.Ext(f.name)))
				if err != nil {
					t.Fatalf("no handler for %s: %v", f.name, err)
				}
				once, err := h.Format(f.data)
				if err != nil {
					t.Fatalf("Format: %v", err)
				}
				twice, err := h.Format(once)
				if err != nil {
					t.Fatalf("Format (second pass): %v", err)
				}
				if string(twice) != string(once) {
					t.Error("not idempotent")
				}
			})
		}
	}
}
