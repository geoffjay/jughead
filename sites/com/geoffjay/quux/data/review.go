package data

// review.go exposes accessor functions over the fixture data. These are the
// only entry points the quux handlers call, so swapping fixtures for a real
// GitHub API client later means replacing this file alone.

// GetReviewState returns the full demo state: all assigned PRs and the first
// PR selected as the default view, with its associated collaborators,
// contributors, and agents populated.
func GetReviewState() ReviewState {
	prs := append([]PullRequest(nil), allPRs...)
	state := ReviewState{PRs: prs}
	if len(prs) > 0 {
		state.SelectedPR = &prs[0]
		state.Collaborators = collaboratorsFor(&prs[0])
		state.Contributors = contributorsFor(&prs[0])
		state.Agents = agentsFor(&prs[0])
	}
	return state
}

// GetReviewStateForPR returns the state with a specific PR selected, or nil if
// no PR with that number exists. Used by the HTMX partial endpoint.
func GetReviewStateForPR(number int) *ReviewState {
	state := GetReviewState()
	return state.SelectPR(number)
}
