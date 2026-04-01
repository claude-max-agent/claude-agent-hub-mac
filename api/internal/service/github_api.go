package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// GitHubAPISummary represents the aggregated GitHub summary response
type GitHubAPISummary struct {
	Repos []GitHubRepoSummary `json:"repos"`
}

// GitHubRepoSummary represents GitHub data for a single repository
type GitHubRepoSummary struct {
	Repo          string              `json:"repo"`
	RecentPRs     []GitHubPRSummary   `json:"recent_prs"`
	RecentCommits []GitHubCommitSummary `json:"recent_commits"`
	OpenIssues    int                 `json:"open_issues"`
}

// GitHubPRSummary represents a PR in the summary
type GitHubPRSummary struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	User      string    `json:"user"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Merged    bool      `json:"merged"`
}

// GitHubCommitSummary represents a commit in the summary
type GitHubCommitSummary struct {
	SHA     string    `json:"sha"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
	HTMLURL string    `json:"html_url"`
}

// GitHubAPIService handles GitHub REST API calls with caching
type GitHubAPIService struct {
	token      string
	repos      []string
	httpClient *http.Client
	cache      *githubCache
	useGH      bool // true if gh command is authenticated
}

type githubCache struct {
	mu        sync.RWMutex
	data      *GitHubAPISummary
	fetchedAt time.Time
	ttl       time.Duration
}

func (c *githubCache) get() (*GitHubAPISummary, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.data == nil {
		return nil, false
	}
	if time.Since(c.fetchedAt) > c.ttl {
		return c.data, false // stale but return for fallback
	}
	return c.data, true
}

func (c *githubCache) set(data *GitHubAPISummary) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = data
	c.fetchedAt = time.Now()
}

func (c *githubCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = nil
	c.fetchedAt = time.Time{}
}

// NewGitHubAPIService creates a new GitHub API service
func NewGitHubAPIService(repos []string) *GitHubAPIService {
	token := os.Getenv("GITHUB_TOKEN")

	// Check if gh command is authenticated
	useGH := isGHAuthenticated()

	return &GitHubAPIService{
		token: token,
		repos: repos,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: &githubCache{
			ttl: 60 * time.Second,
		},
		useGH: useGH,
	}
}

// isGHAuthenticated checks if gh command is authenticated
func isGHAuthenticated() bool {
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// IsConfigured returns true if the GitHub token is set or gh command is authenticated
func (s *GitHubAPIService) IsConfigured() bool {
	return s.useGH || s.token != ""
}

// GetConfigStatus returns a human-readable configuration status
func (s *GitHubAPIService) GetConfigStatus() string {
	if s.useGH && s.token != "" {
		return "gh command (primary) + GITHUB_TOKEN (fallback)"
	} else if s.useGH {
		return "gh command"
	} else if s.token != "" {
		return "GITHUB_TOKEN"
	}
	return "not configured"
}

// GetSummary returns the aggregated GitHub summary for all configured repos
func (s *GitHubAPIService) GetSummary() (*GitHubAPISummary, error) {
	// Check cache first
	if cached, valid := s.cache.get(); valid {
		return cached, nil
	}

	// If token not configured, return empty summary
	if !s.IsConfigured() {
		return &GitHubAPISummary{Repos: []GitHubRepoSummary{}}, nil
	}

	repos := s.getRepoList()
	if len(repos) == 0 {
		return &GitHubAPISummary{Repos: []GitHubRepoSummary{}}, nil
	}

	// Fetch data for all repos concurrently
	type repoResult struct {
		summary GitHubRepoSummary
		err     error
	}
	results := make(chan repoResult, len(repos))

	for _, repo := range repos {
		go func(repoFullName string) {
			summary, err := s.fetchRepoSummary(repoFullName)
			results <- repoResult{summary: summary, err: err}
		}(repo)
	}

	var summaries []GitHubRepoSummary
	for range repos {
		result := <-results
		if result.err != nil {
			log.Printf("[GitHub API] Error fetching repo data: %v", result.err)
			continue
		}
		summaries = append(summaries, result.summary)
	}

	// Sort repos alphabetically for stable ordering
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Repo < summaries[j].Repo
	})

	summary := &GitHubAPISummary{Repos: summaries}
	s.cache.set(summary)

	return summary, nil
}

// RefreshSummary clears cache and fetches fresh data from GitHub
func (s *GitHubAPIService) RefreshSummary() (*GitHubAPISummary, error) {
	s.cache.clear()
	return s.GetSummary()
}

// getRepoList returns all unique repo full names configured for summary fetch
func (s *GitHubAPIService) getRepoList() []string {
	seen := make(map[string]bool)
	var repos []string
	for _, repo := range s.repos {
		repo = strings.TrimSpace(repo)
		if repo == "" || seen[repo] {
			continue
		}
		seen[repo] = true
		repos = append(repos, repo)
	}
	return repos
}

// fetchRepoSummary fetches PRs, commits, and open issue count for a single repo
func (s *GitHubAPIService) fetchRepoSummary(repoFullName string) (GitHubRepoSummary, error) {
	summary := GitHubRepoSummary{
		Repo:          repoFullName,
		RecentPRs:     []GitHubPRSummary{},
		RecentCommits: []GitHubCommitSummary{},
	}

	// Fetch concurrently: PRs, commits, open issues
	type fetchResult struct {
		kind string
		data interface{}
		err  error
	}
	ch := make(chan fetchResult, 3)

	go func() {
		var prs []GitHubPRSummary
		var err error
		if s.useGH {
			prs, err = s.fetchRecentPRsWithGH(repoFullName)
			// Fallback to HTTP API if gh command fails
			if err != nil {
				log.Printf("[GitHub API] gh command failed for PRs, falling back to HTTP API: %v", err)
				prs, err = s.fetchRecentPRs(repoFullName)
			}
		} else {
			prs, err = s.fetchRecentPRs(repoFullName)
		}
		ch <- fetchResult{kind: "prs", data: prs, err: err}
	}()
	go func() {
		var commits []GitHubCommitSummary
		var err error
		if s.useGH {
			commits, err = s.fetchRecentCommitsWithGH(repoFullName)
			// Fallback to HTTP API if gh command fails
			if err != nil {
				log.Printf("[GitHub API] gh command failed for commits, falling back to HTTP API: %v", err)
				commits, err = s.fetchRecentCommits(repoFullName)
			}
		} else {
			commits, err = s.fetchRecentCommits(repoFullName)
		}
		ch <- fetchResult{kind: "commits", data: commits, err: err}
	}()
	go func() {
		var count int
		var err error
		if s.useGH {
			count, err = s.fetchOpenIssueCountWithGH(repoFullName)
			// Fallback to HTTP API if gh command fails
			if err != nil {
				log.Printf("[GitHub API] gh command failed for issues, falling back to HTTP API: %v", err)
				count, err = s.fetchOpenIssueCount(repoFullName)
			}
		} else {
			count, err = s.fetchOpenIssueCount(repoFullName)
		}
		ch <- fetchResult{kind: "issues", data: count, err: err}
	}()

	for i := 0; i < 3; i++ {
		result := <-ch
		if result.err != nil {
			log.Printf("[GitHub API] Error fetching %s for %s: %v", result.kind, repoFullName, result.err)
			continue
		}
		switch result.kind {
		case "prs":
			if prs, ok := result.data.([]GitHubPRSummary); ok {
				summary.RecentPRs = prs
			}
		case "commits":
			if commits, ok := result.data.([]GitHubCommitSummary); ok {
				summary.RecentCommits = commits
			}
		case "issues":
			if count, ok := result.data.(int); ok {
				summary.OpenIssues = count
			}
		}
	}

	return summary, nil
}

// fetchRecentPRs fetches the 5 most recent PRs for a repo
func (s *GitHubAPIService) fetchRecentPRs(repoFullName string) ([]GitHubPRSummary, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls?state=all&sort=updated&direction=desc&per_page=5", repoFullName)

	var apiPRs []struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		HTMLURL   string `json:"html_url"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
	}

	if err := s.githubGet(url, &apiPRs); err != nil {
		return nil, err
	}

	prs := make([]GitHubPRSummary, len(apiPRs))
	for i, pr := range apiPRs {
		prs[i] = GitHubPRSummary{
			Number:    pr.Number,
			Title:     pr.Title,
			State:     pr.State,
			User:      pr.User.Login,
			HTMLURL:   pr.HTMLURL,
			CreatedAt: pr.CreatedAt,
			UpdatedAt: pr.UpdatedAt,
			Merged:    pr.MergedAt != nil,
		}
	}
	return prs, nil
}

// fetchRecentCommits fetches the 5 most recent commits on the default branch
func (s *GitHubAPIService) fetchRecentCommits(repoFullName string) ([]GitHubCommitSummary, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/commits?per_page=5", repoFullName)

	var apiCommits []struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
		Commit  struct {
			Message string `json:"message"`
			Author  struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}

	if err := s.githubGet(url, &apiCommits); err != nil {
		return nil, err
	}

	commits := make([]GitHubCommitSummary, len(apiCommits))
	for i, c := range apiCommits {
		msg := c.Commit.Message
		// Truncate to first line
		for j, ch := range msg {
			if ch == '\n' {
				msg = msg[:j]
				break
			}
		}
		commits[i] = GitHubCommitSummary{
			SHA:     c.SHA,
			Message: msg,
			Author:  c.Commit.Author.Name,
			Date:    c.Commit.Author.Date,
			HTMLURL: c.HTMLURL,
		}
	}
	return commits, nil
}

// fetchOpenIssueCount fetches the count of open issues (excluding PRs) for a repo
func (s *GitHubAPIService) fetchOpenIssueCount(repoFullName string) (int, error) {
	// GitHub API /repos/{owner}/{repo} returns open_issues_count (includes PRs)
	// To get only issues, we use search API
	url := fmt.Sprintf("https://api.github.com/search/issues?q=repo:%s+type:issue+state:open&per_page=1", repoFullName)

	var searchResult struct {
		TotalCount int `json:"total_count"`
	}

	if err := s.githubGet(url, &searchResult); err != nil {
		return 0, err
	}
	return searchResult.TotalCount, nil
}

// githubGet performs an authenticated GET request to the GitHub API
func (s *GitHubAPIService) githubGet(url string, result interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "claude-agent-hub")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned %d for %s: %s", resp.StatusCode, url, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response from %s: %w", url, err)
	}

	return nil
}

// fetchRecentPRsWithGH fetches the 5 most recent PRs using gh command
func (s *GitHubAPIService) fetchRecentPRsWithGH(repoFullName string) ([]GitHubPRSummary, error) {
	cmd := exec.Command("gh", "api",
		fmt.Sprintf("/repos/%s/pulls?state=all&sort=updated&direction=desc&per_page=5", repoFullName))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh api command failed: %w", err)
	}

	var apiPRs []struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		HTMLURL   string `json:"html_url"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
	}

	if err := json.Unmarshal(output, &apiPRs); err != nil {
		return nil, fmt.Errorf("failed to parse gh api output: %w", err)
	}

	prs := make([]GitHubPRSummary, len(apiPRs))
	for i, pr := range apiPRs {
		prs[i] = GitHubPRSummary{
			Number:    pr.Number,
			Title:     pr.Title,
			State:     pr.State,
			User:      pr.User.Login,
			HTMLURL:   pr.HTMLURL,
			CreatedAt: pr.CreatedAt,
			UpdatedAt: pr.UpdatedAt,
			Merged:    pr.MergedAt != nil,
		}
	}
	return prs, nil
}

// fetchRecentCommitsWithGH fetches the 5 most recent commits using gh command
func (s *GitHubAPIService) fetchRecentCommitsWithGH(repoFullName string) ([]GitHubCommitSummary, error) {
	cmd := exec.Command("gh", "api",
		fmt.Sprintf("/repos/%s/commits?per_page=5", repoFullName))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh api command failed: %w", err)
	}

	var apiCommits []struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
		Commit  struct {
			Message string `json:"message"`
			Author  struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}

	if err := json.Unmarshal(output, &apiCommits); err != nil {
		return nil, fmt.Errorf("failed to parse gh api output: %w", err)
	}

	commits := make([]GitHubCommitSummary, len(apiCommits))
	for i, c := range apiCommits {
		msg := c.Commit.Message
		// Truncate to first line
		if idx := strings.Index(msg, "\n"); idx != -1 {
			msg = msg[:idx]
		}
		commits[i] = GitHubCommitSummary{
			SHA:     c.SHA,
			Message: msg,
			Author:  c.Commit.Author.Name,
			Date:    c.Commit.Author.Date,
			HTMLURL: c.HTMLURL,
		}
	}
	return commits, nil
}

// fetchOpenIssueCountWithGH fetches the count of open issues using gh command
func (s *GitHubAPIService) fetchOpenIssueCountWithGH(repoFullName string) (int, error) {
	cmd := exec.Command("gh", "api",
		fmt.Sprintf("/search/issues?q=repo:%s+type:issue+state:open&per_page=1", repoFullName))

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("gh api command failed: %w", err)
	}

	var searchResult struct {
		TotalCount int `json:"total_count"`
	}

	if err := json.Unmarshal(output, &searchResult); err != nil {
		return 0, fmt.Errorf("failed to parse gh api output: %w", err)
	}

	return searchResult.TotalCount, nil
}
