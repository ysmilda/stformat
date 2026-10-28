package main

import (
	"strings"
	"testing"
)

// TestUnifiedDiff checks the -diff output: a unified diff that patch and git
// apply read, aligned so that an inserted line does not shift the rest.
func TestUnifiedDiff(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		old  string
		new  string
		want string
	}{
		{
			name: "unchanged",
			old:  "a\nb\n",
			new:  "a\nb\n",
			want: "",
		},
		{
			name: "one line changed",
			old:  "a\nb\nc\n",
			new:  "a\nB\nc\n",
			want: "--- a/f.st\n+++ b/f.st\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n",
		},
		{
			name: "inserted blank line does not shift the rest",
			old:  "a\nb\nc\nd\n",
			new:  "a\nb\n\nc\nd\n",
			want: "--- a/f.st\n+++ b/f.st\n@@ -1,4 +1,5 @@\n a\n b\n+\n c\n d\n",
		},
		{
			name: "deleted line",
			old:  "a\nb\nc\n",
			new:  "a\nc\n",
			want: "--- a/f.st\n+++ b/f.st\n@@ -1,3 +1,2 @@\n a\n-b\n c\n",
		},
		{
			name: "trailing newline added",
			old:  "a",
			new:  "a\n",
			want: "--- a/f.st\n+++ b/f.st\n@@ -1,1 +1,1 @@\n-a\n\\ No newline at end of file\n+a\n",
		},
		{
			name: "insertion into an empty file",
			old:  "",
			new:  "a\n",
			want: "--- a/f.st\n+++ b/f.st\n@@ -0,0 +1,1 @@\n+a\n",
		},
		{
			name: "distant changes make separate hunks",
			old:  "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n",
			new:  "one\n2\n3\n4\n5\n6\n7\n8\n9\nten\n",
			want: "--- a/f.st\n+++ b/f.st\n" +
				"@@ -1,4 +1,4 @@\n-1\n+one\n 2\n 3\n 4\n" +
				"@@ -7,4 +7,4 @@\n 7\n 8\n 9\n-10\n+ten\n",
		},
		{
			name: "close changes share a hunk",
			old:  "1\n2\n3\n4\n5\n",
			new:  "one\n2\n3\n4\nfive\n",
			want: "--- a/f.st\n+++ b/f.st\n@@ -1,5 +1,5 @@\n-1\n+one\n 2\n 3\n 4\n-5\n+five\n",
		},
		{
			name: "line endings are kept",
			old:  "a\r\nb\r\n",
			new:  "a\r\nB\r\n",
			want: "--- a/f.st\n+++ b/f.st\n@@ -1,2 +1,2 @@\n a\r\n-b\r\n+B\r\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := unifiedDiff("f.st", c.old, c.new); got != c.want {
				t.Errorf("unifiedDiff()\n got: %q\nwant: %q", got, c.want)
			}
		})
	}
}

// TestUnifiedDiffPath checks that a Windows path is written with forward
// slashes, as patch tools expect.
func TestUnifiedDiffPath(t *testing.T) {
	t.Parallel()
	got := unifiedDiff(`src\plc\f.st`, "a\n", "b\n")
	if !strings.HasPrefix(got, "--- a/src/plc/f.st\n+++ b/src/plc/f.st\n") {
		t.Errorf("header = %q", got)
	}
}

// TestEditScriptLargeMiddle checks that a file whose middle is too big to
// align still gets a correct script instead of a huge table.
func TestEditScriptLargeMiddle(t *testing.T) {
	t.Parallel()
	old := make([]string, maxDiffCells)
	new := make([]string, maxDiffCells)
	for i := range old {
		old[i] = "same line\n"
		new[i] = "changed line\n"
	}
	got := editScript(old, new)
	if len(got) != len(old)+len(new) {
		t.Errorf("fallback produced %d edits, want %d", len(got), len(old)+len(new))
	}
}
