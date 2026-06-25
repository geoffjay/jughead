// Package patch parses the unified-diff `patch` field returned by GitHub's
// /repos/{owner}/{repo}/pulls/{number}/files endpoint into structured hunks
// suitable for a side-by-side diff view.
//
// The parser is a pure function operating on a string; it has no I/O
// dependencies and does not import the data package, avoiding an import
// cycle. Callers map the returned Hunk/DiffLine values into their own model.
package patch

import (
	"fmt"
	"strconv"
	"strings"
)

// LineKind classifies a single line within a hunk.
type LineKind int

const (
	LineContext LineKind = iota
	LineAdded
	LineRemoved
)

// DiffLine is one line of a hunk. OldLineNo is the pre-change line number
// (0 for added lines); NewLineNo is the post-change line number (0 for
// removed lines).
type DiffLine struct {
	Kind      LineKind
	OldLineNo int
	NewLineNo int
	Text      string
}

// Hunk is a contiguous run of changed lines with surrounding context.
type Hunk struct {
	OldStart int
	NewStart int
	Lines    []DiffLine
}

const (
	// minimumHeaderFields is the minimum number of whitespace-delimited fields
	// expected in a hunk header after trimming the "@@ ... @@" delimiters, e.g.
	// "-42,7 +50,9" has 2 fields.
	minimumHeaderFields = 2
)

// Parse converts a unified-diff patch string into a slice of Hunks. It
// recognizes hunk headers of the form
//
//	@@ -oldStart[,oldCount] +newStart[,newCount] @@[ optional context]
//
// and body lines beginning with ' ' (context), '+' (added), or '-' (removed).
// The leading "diff --git", "---", "+++", and "index" control lines are
// skipped; only hunk bodies are parsed. Lines starting with "\\" (e.g.
// "\\ No newline at end of file") are treated as metadata and ignored.
func Parse(patch string) ([]Hunk, error) {
	lines := splitLines(patch)
	var hunks []Hunk
	i := 0
	for i < len(lines) {
		raw := lines[i]
		if isHunkHeader(raw) {
			hunk, next, err := parseHunk(lines, i)
			if err != nil {
				return nil, err
			}
			hunks = append(hunks, hunk)
			i = next
			continue
		}
		// Skip control lines (diff --git, ---, +++, index, etc.) and blank
		// lines between hunks.
		i++
	}
	return hunks, nil
}

// parseHunk parses a single hunk starting at lines[idx], which must be a hunk
// header. Returns the parsed Hunk and the index of the first line *after* the
// hunk body.
func parseHunk(lines []string, idx int) (Hunk, int, error) {
	header := lines[idx]
	oldStart, newStart, err := parseHunkHeader(header)
	if err != nil {
		return Hunk{}, idx, fmt.Errorf("patch: hunk header at line %d: %w", idx+1, err)
	}

	hunk := Hunk{OldStart: oldStart, NewStart: newStart}
	oldLine, newLine := oldStart, newStart

	i := idx + 1
	for i < len(lines) {
		raw := lines[i]
		// A new hunk header or diff control line ends the current hunk body.
		if isHunkHeader(raw) || isControlLine(raw) {
			break
		}
		if isNoNewlineMarker(raw) {
			i++
			continue
		}
		line, err := parseDiffLine(raw, &oldLine, &newLine)
		if err != nil {
			return Hunk{}, i, fmt.Errorf("patch: line %d: %w", i+1, err)
		}
		hunk.Lines = append(hunk.Lines, line)
		i++
	}
	return hunk, i, nil
}

// parseDiffLine interprets the leading marker byte (' ', '+', '-') of a body
// line and advances oldLine/newLine counters accordingly.
func parseDiffLine(raw string, oldLine, newLine *int) (DiffLine, error) {
	if raw == "" {
		// Treat an empty line as context (some tools emit bare newlines).
		*oldLine++
		*newLine++
		return DiffLine{Kind: LineContext, OldLineNo: *oldLine - 1, NewLineNo: *newLine - 1, Text: ""}, nil
	}

	marker := raw[0]
	text := raw[1:] // drop the marker byte

	switch marker {
	case ' ':
		*oldLine++
		*newLine++
		return DiffLine{Kind: LineContext, OldLineNo: *oldLine - 1, NewLineNo: *newLine - 1, Text: text}, nil
	case '+':
		*newLine++
		return DiffLine{Kind: LineAdded, OldLineNo: 0, NewLineNo: *newLine - 1, Text: text}, nil
	case '-':
		*oldLine++
		return DiffLine{Kind: LineRemoved, OldLineNo: *oldLine - 1, NewLineNo: 0, Text: text}, nil
	default:
		return DiffLine{}, fmt.Errorf("unexpected line marker %q", string(marker))
	}
}

// parseHunkHeader extracts the oldStart and newStart line numbers from a hunk
// header of the form "@@ -oldStart[,oldCount] +newStart[,newCount] @@".
func parseHunkHeader(header string) (oldStart, newStart int, err error) {
	// Trim the leading "@@ " and the trailing "@@" plus any context label.
	s := strings.TrimPrefix(header, "@@")
	if endIdx := strings.Index(s, "@@"); endIdx >= 0 {
		s = s[:endIdx]
	}
	s = strings.TrimSpace(s)

	// Now s looks like "-42,7 +50,9".
	parts := strings.Fields(s)
	if len(parts) < minimumHeaderFields {
		return 0, 0, fmt.Errorf("malformed hunk header %q", header)
	}

	oldStart, err = parseStart(parts[0])
	if err != nil {
		return 0, 0, err
	}
	newStart, err = parseStart(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return oldStart, newStart, nil
}

// parseStart parses a "-42,7" or "+50" segment into its starting line number.
func parseStart(seg string) (int, error) {
	seg = strings.TrimLeft(seg, "-+")
	if commaIdx := strings.IndexByte(seg, ','); commaIdx >= 0 {
		seg = seg[:commaIdx]
	}
	n, err := strconv.Atoi(seg)
	if err != nil {
		return 0, fmt.Errorf("invalid line number %q: %w", seg, err)
	}
	return n, nil
}

// isHunkHeader reports whether line begins a hunk ("@@ ... @@").
func isHunkHeader(line string) bool {
	return strings.HasPrefix(line, "@@")
}

// controlPrefixes lists the diff control headers that should be skipped. A
// hunk body line never starts with these tokens.
var controlPrefixes = []string{
	"diff --git",
	"---",
	"+++",
	"index ",
	"similarity index",
	"dissimilarity index",
	"rename from",
	"rename to",
	"copy from",
	"copy to",
	"new file mode",
	"deleted file mode",
	"old mode",
	"new mode",
	"Binary files",
}

// isControlLine reports whether line is a diff control header that should be
// skipped.
func isControlLine(line string) bool {
	for _, p := range controlPrefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

// isNoNewlineMarker reports whether line is the "\\ No newline at end of
// file" metadata marker.
func isNoNewlineMarker(line string) bool {
	return strings.HasPrefix(line, "\\ ")
}

// splitLines splits s on '\n' without allocating a trailing empty element when
// s ends with a newline (matching how patch text is typically formatted).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
