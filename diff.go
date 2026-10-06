package main

// diffLines returns the lines that differ between old and new, each prefixed
// with "-" for a removed line and "+" for an added one, in output order.
//
// A plain positional comparison would report every line after an inserted or
// deleted line as changed, which is exactly what the formatter does to a file
// when it inserts a blank line. So the common prefix and suffix are trimmed
// first and only the differing middle is aligned, with a longest common
// subsequence.
func diffLines(old, new []string) []string {
	start := commonPrefix(old, new)
	end := commonSuffix(old, new, min(len(old), len(new))-start)
	midOld, midNew := old[start:len(old)-end], new[start:len(new)-end]
	if len(midOld) == 0 && len(midNew) == 0 {
		return nil
	}

	var out []string
	if len(midOld)*len(midNew) > maxDiffCells {
		// Too big to align exactly; fall back to a positional comparison,
		// which is still no worse than comparing the whole file.
		for i := range max(len(midOld), len(midNew)) {
			if i < len(midOld) {
				out = append(out, "- "+midOld[i])
			}
			if i < len(midNew) {
				out = append(out, "+ "+midNew[i])
			}
		}
		return out
	}

	// table[i][j] is the LCS length of midOld[i:] and midNew[j:].
	table := make([][]int, len(midOld)+1)
	for i := range table {
		table[i] = make([]int, len(midNew)+1)
	}
	for i := len(midOld) - 1; i >= 0; i-- {
		for j := len(midNew) - 1; j >= 0; j-- {
			switch {
			case midOld[i] == midNew[j]:
				table[i][j] = table[i+1][j+1] + 1
			case table[i+1][j] >= table[i][j+1]:
				table[i][j] = table[i+1][j]
			default:
				table[i][j] = table[i][j+1]
			}
		}
	}

	i, j := 0, 0
	for i < len(midOld) && j < len(midNew) {
		switch {
		case midOld[i] == midNew[j]:
			i, j = i+1, j+1
		case table[i+1][j] >= table[i][j+1]:
			out = append(out, "- "+midOld[i])
			i++
		default:
			out = append(out, "+ "+midNew[j])
			j++
		}
	}
	for ; i < len(midOld); i++ {
		out = append(out, "- "+midOld[i])
	}
	for ; j < len(midNew); j++ {
		out = append(out, "+ "+midNew[j])
	}
	return out
}

// maxDiffCells bounds the LCS table, so a file that was largely rewritten does
// not turn the diff into a multi-megabyte allocation.
const maxDiffCells = 4 << 20

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
