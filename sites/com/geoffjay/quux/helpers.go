package quux

import (
	"github.com/geoffjay/jughead/sites/com/geoffjay/quux/data"
	"github.com/geoffjay/jughead/sites/links"
	"github.com/geoffjay/jughead/templates/components/daisyui"
)

// navbarData builds the quux navbar. The "Items" dropdown is replaced with a
// single "About" link now that the home page is the review UI.
func navbarData(r links.LinkResolver) daisyui.NavbarData {
	return daisyui.NavbarData{
		Title:     "Quux Reviews",
		TitleHref: "",
		Resolver:  r,
		End: []daisyui.NavItem{
			{Label: "About", Href: "/about"},
		},
	}
}

// toDiffViewConfig maps the data-layer file diffs into the daisyui DiffView
// component's plain input structs. This keeps the component free of any
// dependency on the quux data model.
func toDiffViewConfig(files []data.FileDiff) daisyui.DiffViewConfig {
	out := daisyui.DiffViewConfig{}
	for _, f := range files {
		df := daisyui.DiffViewFile{
			Path:    f.Path,
			OldPath: f.OldPath,
			Added:   f.Added,
			Removed: f.Removed,
		}
		for _, h := range f.Hunks {
			dh := daisyui.DiffViewHunk{OldStart: h.OldStart, NewStart: h.NewStart}
			for _, l := range h.Lines {
				dh.Lines = append(dh.Lines, daisyui.DiffViewLine{
					Kind:      daisyui.DiffViewLineKind(l.Kind),
					OldLineNo: l.OldLineNo,
					NewLineNo: l.NewLineNo,
					Text:      l.Text,
				})
			}
			df.Hunks = append(df.Hunks, dh)
		}
		out.Files = append(out.Files, df)
	}
	return out
}

// statusBadge returns the full badge class for a PR status, e.g.
// "badge badge-primary". Safe to drop straight into a templ class attribute.
func statusBadge(s data.PRStatus) string {
	return "badge " + statusBadgeClass(s)
}

// statusBadgeClass maps a PR status to a daisyUI badge color suffix class
// (e.g. "badge-primary").
func statusBadgeClass(s data.PRStatus) string {
	switch s {
	case data.PRStatusOpen:
		return "badge-primary"
	case data.PRStatusDraft:
		return "badge-ghost"
	case data.PRStatusReviewing:
		return "badge-info"
	case data.PRStatusApproved:
		return "badge-success"
	case data.PRStatusChangesReq:
		return "badge-warning"
	case data.PRStatusMerged:
		return "badge-secondary"
	case data.PRStatusClosed:
		return "badge-error"
	default:
		return "badge-ghost"
	}
}

// statusLabel returns a human-readable label for a PR status.
func statusLabel(s data.PRStatus) string {
	switch s {
	case data.PRStatusOpen:
		return "Open"
	case data.PRStatusDraft:
		return "Draft"
	case data.PRStatusReviewing:
		return "Reviewing"
	case data.PRStatusApproved:
		return "Approved"
	case data.PRStatusChangesReq:
		return "Changes requested"
	case data.PRStatusMerged:
		return "Merged"
	case data.PRStatusClosed:
		return "Closed"
	default:
		return string(s)
	}
}

// initials returns the first letter of each whitespace-separated word in name,
// uppercased, capped at 2 characters. Used for avatar placeholders.
func initials(name string) string {
	maxInitials := 2
	if name == "" {
		return "?"
	}
	out := []rune{}
	for _, w := range splitWords(name) {
		if w == "" {
			continue
		}
		r := rune(w[0])
		if r >= 'a' && r <= 'z' {
			r -= 32
		}
		out = append(out, r)
		if len(out) == maxInitials {
			break
		}
	}
	if len(out) == 0 {
		return "?"
	}
	return string(out)
}

func splitWords(s string) []string {
	var words []string
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			if start >= 0 {
				words = append(words, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		words = append(words, s[start:])
	}
	return words
}

// prsByRepo groups PRs by repository full name, preserving the order repos
// first appear in the PR list.
func prsByRepo(prs []data.PullRequest) []repoGroup {
	var groups []repoGroup
	seen := map[string]int{}
	for i := range prs {
		pr := prs[i]
		idx, ok := seen[pr.Repo.FullName]
		if !ok {
			idx = len(groups)
			seen[pr.Repo.FullName] = idx
			groups = append(groups, repoGroup{Repo: pr.Repo})
		}
		groups[idx].PRs = append(groups[idx].PRs, pr)
	}
	return groups
}

type repoGroup struct {
	Repo data.Repository
	PRs  []data.PullRequest
}
