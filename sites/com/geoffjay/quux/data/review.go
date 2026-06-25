package data

// review.go holds the ReviewState assembly helpers shared by both the
// fixture-backed tests and the real client path. The pure SelectPR selects a
// PR already present in state.PRs without re-fetching people; the real client
// path (in the quux Service) calls githubsvc.Client to populate people afresh.

// SelectPR returns a copy of the state with the PR matching the given number
// selected. It does NOT refresh collaborators/contributors/agents — callers
// that need those populated (the real client path) must set them themselves.
// It returns nil when no PR with that number exists.
func (s *ReviewState) SelectPR(number int) *ReviewState {
	for i := range s.PRs {
		if s.PRs[i].Number != number {
			continue
		}
		pr := s.PRs[i]
		out := *s
		out.SelectedPR = &pr
		return &out
	}
	return nil
}
