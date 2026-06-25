package quux

import (
	"context"
	"fmt"

	"github.com/geoffjay/jughead/auth"
	"github.com/geoffjay/jughead/sites/com/geoffjay/quux/data"

	"github.com/gin-gonic/gin"
)

// Service orchestrates the auth.Client (GitHub REST) and the data.Cache to
// assemble data.ReviewState for the quux handlers. It is the only thing the
// handlers call; the data package stays free of any auth import (avoiding a
// cycle).
//
// The Service holds only the cache (shared, long-lived). The auth.Client is
// built per request from the session's access token (in the github_token gin
// context key) so each user exercises their own GitHub permissions.
type Service struct {
	cache *data.Cache
}

// NewService builds a Service wired to the given cache. The auth.Client is
// constructed per-request via clientFor.
func NewService(cache *data.Cache) *Service {
	return &Service{cache: cache}
}

// clientFor builds an auth.Client from the session token stashed in the gin
// context by middleware.GitHubAuthRequired. The middleware guarantees the key
// is present and is a string.
func clientFor(c *gin.Context) *auth.Client {
	token, _ := c.Get("github_token")
	return auth.NewClient(token.(string))
}

// LoadReviewState fetches the viewer's assigned PRs and selects the first one,
// populating its collaborators, contributors, and agents via the cache.
func (s *Service) LoadReviewState(ctx context.Context, client *auth.Client, viewerLogin string) (data.ReviewState, error) {
	prs, err := s.loadPRs(ctx, client, viewerLogin)
	if err != nil {
		return data.ReviewState{}, err
	}
	state := data.ReviewState{PRs: prs}
	if len(prs) == 0 {
		return state, nil
	}
	if err := s.populateForPR(ctx, client, &state, &prs[0]); err != nil {
		return data.ReviewState{}, err
	}
	return state, nil
}

// LoadReviewStateForPR fetches the assigned PRs and selects the one matching
// number, refreshing the detail (files + branches + status) and people.
func (s *Service) LoadReviewStateForPR(
	ctx context.Context,
	client *auth.Client,
	viewerLogin, repoOwner, repoName string,
	number int,
) (*data.ReviewState, error) {
	prs, err := s.loadPRs(ctx, client, viewerLogin)
	if err != nil {
		return nil, err
	}
	state := data.ReviewState{PRs: prs}

	pr, err := s.loadPRDetail(ctx, client, repoOwner, repoName, number)
	if err != nil {
		return nil, err
	}
	state.SelectedPR = &pr
	if err := s.populateForPR(ctx, client, &state, &pr); err != nil {
		return nil, err
	}
	return &state, nil
}

// loadPRs returns the cached-or-fetched assigned PRs.
func (s *Service) loadPRs(ctx context.Context, client *auth.Client, viewerLogin string) ([]data.PullRequest, error) {
	key := "prs:" + viewerLogin
	v, err := s.cache.Get(key, data.PrListTTL, func() (any, error) {
		return client.ListAssignedPRs(ctx, viewerLogin)
	})
	if err != nil {
		return nil, fmt.Errorf("load assigned PRs: %w", err)
	}
	return v.([]data.PullRequest), nil
}

// loadPRDetail returns the cached-or-fetched full PR detail.
func (s *Service) loadPRDetail(ctx context.Context, client *auth.Client, owner, repo string, number int) (data.PullRequest, error) {
	key := fmt.Sprintf("pr:%s/%s:%d", owner, repo, number)
	v, err := s.cache.Get(key, data.PrDetailTTL, func() (any, error) {
		return client.LoadPR(ctx, owner, repo, number)
	})
	if err != nil {
		return data.PullRequest{}, fmt.Errorf("load PR detail: %w", err)
	}
	return v.(data.PullRequest), nil
}

// populateForPR fills the SelectedPR + people sections of state from the cache.
func (s *Service) populateForPR(ctx context.Context, client *auth.Client, state *data.ReviewState, pr *data.PullRequest) error {
	state.SelectedPR = pr

	collabs, err := s.loadCollaborators(ctx, client, pr.Repo.Owner, pr.Repo.Name)
	if err != nil {
		return err
	}
	state.Collaborators = collabs

	contribs, err := s.loadContributors(ctx, client, pr.Repo.Owner, pr.Repo.Name)
	if err != nil {
		return err
	}
	state.Contributors = contribs

	agents, err := s.loadAgents(ctx, client, pr.Repo.Owner, pr.Repo.Name)
	if err != nil {
		return err
	}
	state.Agents = agents
	return nil
}

func (s *Service) loadCollaborators(ctx context.Context, client *auth.Client, owner, repo string) ([]data.User, error) {
	key := "collabs:" + owner + "/" + repo
	v, err := s.cache.Get(key, data.CollabTTL, func() (any, error) {
		return client.ListCollaborators(ctx, owner, repo)
	})
	if err != nil {
		return nil, fmt.Errorf("load collaborators: %w", err)
	}
	return v.([]data.User), nil
}

func (s *Service) loadContributors(ctx context.Context, client *auth.Client, owner, repo string) ([]data.User, error) {
	key := "contribs:" + owner + "/" + repo
	v, err := s.cache.Get(key, data.ContribTTL, func() (any, error) {
		return client.ListContributors(ctx, owner, repo)
	})
	if err != nil {
		return nil, fmt.Errorf("load contributors: %w", err)
	}
	return v.([]data.User), nil
}

func (s *Service) loadAgents(ctx context.Context, client *auth.Client, owner, repo string) ([]data.User, error) {
	key := "agents:" + owner + "/" + repo
	v, err := s.cache.Get(key, data.AgentTTL, func() (any, error) {
		return client.ListAgents(ctx, owner, repo)
	})
	if err != nil {
		return nil, fmt.Errorf("load agents: %w", err)
	}
	return v.([]data.User), nil
}
