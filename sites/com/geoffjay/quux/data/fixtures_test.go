package data

import "testing"

func TestGetReviewStateSelectsFirstPR(t *testing.T) {
	state := GetReviewState()
	if len(state.PRs) == 0 {
		t.Fatal("expected fixture PRs, got none")
	}
	if state.SelectedPR == nil {
		t.Fatal("expected a selected PR by default, got nil")
	}
	if state.SelectedPR.Number != state.PRs[0].Number {
		t.Errorf("selected PR = #%d, want #%d", state.SelectedPR.Number, state.PRs[0].Number)
	}
	if len(state.Collaborators) == 0 {
		t.Error("expected collaborators populated for the default PR")
	}
	if len(state.Contributors) == 0 {
		t.Error("expected contributors populated for the default PR")
	}
	if len(state.Agents) == 0 {
		t.Error("expected agents populated for the default PR")
	}
}

func TestGetReviewStateForPR(t *testing.T) {
	if got := GetReviewStateForPR(42); got == nil || got.SelectedPR == nil || got.SelectedPR.Number != 42 {
		t.Errorf("GetReviewStateForPR(42) = %#v, want selected PR #42", got)
	}
	if got := GetReviewStateForPR(999); got != nil {
		t.Errorf("GetReviewStateForPR(999) = %#v, want nil for unknown PR", got)
	}
}

func TestSelectPRRefreshesPeople(t *testing.T) {
	state := GetReviewState()
	next := state.SelectPR(17)
	if next == nil || next.SelectedPR == nil || next.SelectedPR.Number != 17 {
		t.Fatalf("SelectPR(17) = %#v, want selected PR #17", next)
	}
	if len(next.Collaborators) == 0 || len(next.Contributors) == 0 || len(next.Agents) == 0 {
		t.Error("expected people lists refreshed for the selected PR")
	}
}

func TestPRsCoverMultipleRepos(t *testing.T) {
	state := GetReviewState()
	seen := map[string]bool{}
	for _, pr := range state.PRs {
		seen[pr.Repo.FullName] = true
	}
	if len(seen) < 2 {
		t.Errorf("expected PRs from at least 2 repos, got %d", len(seen))
	}
}

func TestFileDiffHasHunks(t *testing.T) {
	state := GetReviewState()
	if state.SelectedPR == nil {
		t.Fatal("no selected PR")
	}
	totalLines := 0
	for _, f := range state.SelectedPR.Files {
		if len(f.Hunks) == 0 {
			t.Errorf("file %s has no hunks", f.Path)
		}
		for _, h := range f.Hunks {
			totalLines += len(h.Lines)
		}
	}
	if totalLines == 0 {
		t.Error("expected diff lines in the selected PR, got none")
	}
}
