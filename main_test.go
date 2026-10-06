package main

import (
	"strings"
	"testing"
)

// TestDiffLines checks the alignment of the -diff output. A positional
// comparison reports every line after an insertion as changed, which the
// formatter triggers whenever it inserts a blank line.
func TestDiffLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		old  string
		new  string
		want []string
	}{
		{
			name: "unchanged",
			old:  "a\nb\n",
			new:  "a\nb\n",
			want: nil,
		},
		{
			name: "one line changed",
			old:  "a\nb\nc\n",
			new:  "a\nB\nc\n",
			want: []string{"- b", "+ B"},
		},
		{
			name: "inserted blank line does not shift the rest",
			old:  "a\nb\nc\nd\n",
			new:  "a\nb\n\nc\nd\n",
			want: []string{"+ "},
		},
		{
			name: "deleted line",
			old:  "a\nb\nc\n",
			new:  "a\nc\n",
			want: []string{"- b"},
		},
		{
			name: "trailing newline added",
			old:  "a",
			new:  "a\n",
			want: []string{"+ "},
		},
		{
			// With repeated lines the diff stays minimal: the last old line
			// matches the trailing new one, so only one line changes.
			name: "repeated lines",
			old:  "x := 1;\nx := 1;\nx := 1;\n",
			new:  "x := 1;\n\nx := 1;\n",
			want: []string{"- x := 1;", "+ "},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := diffLines(strings.Split(c.old, "\n"), strings.Split(c.new, "\n"))
			if len(got) != len(c.want) {
				t.Fatalf("diffLines()\n got: %q\nwant: %q", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("line %d: got %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

// TestDiffLinesLargeMiddle checks that a file whose middle is too big to align
// still produces a usable diff instead of a huge table.
func TestDiffLinesLargeMiddle(t *testing.T) {
	t.Parallel()
	old := make([]string, maxDiffCells)
	new := make([]string, maxDiffCells)
	for i := range old {
		old[i] = "same line"
		new[i] = "changed line"
	}
	got := diffLines(old, new)
	if len(got) != len(old)+len(new) {
		t.Errorf("fallback produced %d lines, want %d", len(got), len(old)+len(new))
	}
}
