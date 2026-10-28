package main

import (
	"fmt"
	"strings"
)

// diffContext is how many unchanged lines a hunk shows around a change.
const diffContext = 3

// maxDiffCells bounds the LCS table, so a file that was largely rewritten does
// not turn the diff into a multi-megabyte allocation.
const maxDiffCells = 4 << 20

// edit is one line of an edit script: kept (' '), removed ('-') or added ('+').
type edit struct {
	kind byte
	line string
}

// unifiedDiff returns the changes from old to new as a unified diff of the file
// at path, in the format patch and git apply read, or "" when there are none.
// Lines keep their own line ending, so a CRLF file diffs as CRLF.
func unifiedDiff(path, old, new string) string {
	edits := editScript(splitLines(old), splitLines(new))

	// before[k] and after[k] count the old and new lines ahead of edits[k].
	before := make([]int, len(edits)+1)
	after := make([]int, len(edits)+1)
	for k, e := range edits {
		before[k+1], after[k+1] = before[k], after[k]
		if e.kind != '+' {
			before[k+1]++
		}
		if e.kind != '-' {
			after[k+1]++
		}
	}

	var b strings.Builder
	name := strings.ReplaceAll(path, "\\", "/")
	for i := 0; i < len(edits); {
		if edits[i].kind == ' ' {
			i++
			continue
		}
		// A hunk runs from the context before this change over every change
		// whose unchanged gap is too short to separate two hunks.
		start, end := max(i-diffContext, 0), i
		for end < len(edits) {
			if edits[end].kind != ' ' {
				end++
				continue
			}
			gap := end
			for gap < len(edits) && edits[gap].kind == ' ' {
				gap++
			}
			if gap == len(edits) || gap-end > 2*diffContext {
				break
			}
			end = gap
		}
		stop := min(end+diffContext, len(edits))

		if b.Len() == 0 {
			fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", name, name)
		}
		fmt.Fprintf(&b, "@@ -%s +%s @@\n",
			hunkRange(before[start], before[stop]-before[start]),
			hunkRange(after[start], after[stop]-after[start]))
		for _, e := range edits[start:stop] {
			b.WriteByte(e.kind)
			b.WriteString(e.line)
			if !strings.HasSuffix(e.line, "\n") {
				b.WriteString("\n\\ No newline at end of file\n")
			}
		}
		i = stop
	}
	return b.String()
}

// hunkRange formats the start and length of one side of a hunk. Lines count
// from 1; an empty side names the line before it, as unified diffs do.
func hunkRange(skipped, n int) string {
	if n == 0 {
		return fmt.Sprintf("%d,0", skipped)
	}
	return fmt.Sprintf("%d,%d", skipped+1, n)
}

// splitLines splits s into lines that keep their line ending; only the last
// one can lack it.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// editScript returns the edits that turn old into new.
//
// A plain positional comparison would report every line after an inserted or
// deleted line as changed, which is exactly what the formatter does to a file
// when it inserts a blank line. So the common prefix and suffix are kept as
// they are and only the differing middle is aligned, with a longest common
// subsequence.
func editScript(old, new []string) []edit {
	start := commonPrefix(old, new)
	end := commonSuffix(old, new, min(len(old), len(new))-start)
	midOld, midNew := old[start:len(old)-end], new[start:len(new)-end]

	edits := make([]edit, 0, len(old)+len(midNew))
	for _, l := range old[:start] {
		edits = append(edits, edit{' ', l})
	}

	if len(midOld)*len(midNew) > maxDiffCells {
		// Too big to align: replace the middle as a whole.
		for _, l := range midOld {
			edits = append(edits, edit{'-', l})
		}
		for _, l := range midNew {
			edits = append(edits, edit{'+', l})
		}
	} else {
		edits = appendLCS(edits, midOld, midNew)
	}

	for _, l := range old[len(old)-end:] {
		edits = append(edits, edit{' ', l})
	}
	return edits
}

// appendLCS appends the edits that turn old into new, aligned on their longest
// common subsequence.
func appendLCS(edits []edit, old, new []string) []edit {
	// table[i][j] is the LCS length of old[i:] and new[j:].
	table := make([][]int, len(old)+1)
	for i := range table {
		table[i] = make([]int, len(new)+1)
	}
	for i := len(old) - 1; i >= 0; i-- {
		for j := len(new) - 1; j >= 0; j-- {
			switch {
			case old[i] == new[j]:
				table[i][j] = table[i+1][j+1] + 1
			case table[i+1][j] >= table[i][j+1]:
				table[i][j] = table[i+1][j]
			default:
				table[i][j] = table[i][j+1]
			}
		}
	}

	i, j := 0, 0
	for i < len(old) && j < len(new) {
		switch {
		case old[i] == new[j]:
			edits = append(edits, edit{' ', old[i]})
			i, j = i+1, j+1
		case table[i+1][j] >= table[i][j+1]:
			edits = append(edits, edit{'-', old[i]})
			i++
		default:
			edits = append(edits, edit{'+', new[j]})
			j++
		}
	}
	for ; i < len(old); i++ {
		edits = append(edits, edit{'-', old[i]})
	}
	for ; j < len(new); j++ {
		edits = append(edits, edit{'+', new[j]})
	}
	return edits
}

// commonPrefix returns the number of leading lines old and new share.
func commonPrefix(old, new []string) int {
	n := min(len(old), len(new))
	for i := range n {
		if old[i] != new[i] {
			return i
		}
	}
	return n
}

// commonSuffix returns the number of trailing lines old and new share, looking
// at most limit lines so it cannot match a line the prefix already consumed.
func commonSuffix(old, new []string, limit int) int {
	n := 0
	for n < limit && old[len(old)-1-n] == new[len(new)-1-n] {
		n++
	}
	return n
}
