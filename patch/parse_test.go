package patch

import (
	"reflect"
	"testing"
)

func TestParseSingleHunk(t *testing.T) {
	patch := `@@ -42,7 +42,9 @@ func main() {
 context line
-removed line
+added line 1
+added line 2
 context line 2
`
	hunks, err := Parse(patch)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	want := Hunk{
		OldStart: 42,
		NewStart: 42,
		Lines: []DiffLine{
			{Kind: LineContext, OldLineNo: 42, NewLineNo: 42, Text: "context line"},
			{Kind: LineRemoved, OldLineNo: 43, NewLineNo: 0, Text: "removed line"},
			{Kind: LineAdded, OldLineNo: 0, NewLineNo: 43, Text: "added line 1"},
			{Kind: LineAdded, OldLineNo: 0, NewLineNo: 44, Text: "added line 2"},
			{Kind: LineContext, OldLineNo: 44, NewLineNo: 45, Text: "context line 2"},
		},
	}
	if !reflect.DeepEqual(hunks[0], want) {
		t.Errorf("hunk mismatch:\ngot  %+v\nwant %+v", hunks[0], want)
	}
}

func TestParseMultipleHunks(t *testing.T) {
	patch := `@@ -1,3 +1,3 @@
 a
-b
+c
@@ -10,3 +10,3 @@
  x
-y
+z
`
	hunks, err := Parse(patch)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
	if hunks[0].OldStart != 1 || hunks[0].NewStart != 1 {
		t.Errorf("hunk 0 starts: old=%d new=%d", hunks[0].OldStart, hunks[0].NewStart)
	}
	if hunks[1].OldStart != 10 || hunks[1].NewStart != 10 {
		t.Errorf("hunk 1 starts: old=%d new=%d", hunks[1].OldStart, hunks[1].NewStart)
	}
	if len(hunks[0].Lines) != 3 {
		t.Errorf("hunk 0 lines: %d", len(hunks[0].Lines))
	}
}

func TestParseSkipsControlLines(t *testing.T) {
	patch := `diff --git a/handlers.go b/handlers.go
index abc..def 100644
--- a/handlers.go
+++ b/handlers.go
@@ -1,2 +1,2 @@
 old
-new
+new
`
	hunks, err := Parse(patch)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if len(hunks[0].Lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(hunks[0].Lines))
	}
}

func TestParseNoNewlineMarker(t *testing.T) {
	patch := `@@ -1,2 +1,2 @@
 a
-b
\ No newline at end of file
+c
`
	hunks, err := Parse(patch)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// The "\ No newline" marker should be skipped, yielding 3 lines.
	if len(hunks[0].Lines) != 3 {
		t.Errorf("expected 3 lines (marker skipped), got %d", len(hunks[0].Lines))
	}
}

func TestParseHunkHeaderWithoutCounts(t *testing.T) {
	patch := `@@ -5 +5,3 @@
 x
+y
+z
`
	hunks, err := Parse(patch)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if hunks[0].OldStart != 5 || hunks[0].NewStart != 5 {
		t.Errorf("starts: old=%d new=%d", hunks[0].OldStart, hunks[0].NewStart)
	}
}

func TestParseEmptyPatch(t *testing.T) {
	hunks, err := Parse("")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(hunks) != 0 {
		t.Errorf("expected 0 hunks, got %d", len(hunks))
	}
}

func TestParsePreservesTabs(t *testing.T) {
	patch := "@@ -1,2 +1,2 @@\n \tindented\n-\told\n+\tnew\n"
	hunks, err := Parse(patch)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(hunks[0].Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(hunks[0].Lines))
	}
	added := hunks[0].Lines[2]
	if added.Text != "\tnew" {
		t.Errorf("added text = %q, want %q", added.Text, "\tnew")
	}
	if added.Kind != LineAdded {
		t.Errorf("expected LineAdded, got %v", added.Kind)
	}
}
