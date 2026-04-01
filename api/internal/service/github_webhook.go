package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// GitHubWebhookService handles incoming GitHub webhook events
type GitHubWebhookService struct {
	Secret            string
	lastAutoUpgradeAt time.Time
	autoUpgradeMu     sync.Mutex
}

// NewGitHubWebhookService creates a new GitHub webhook service
func NewGitHubWebhookService() *GitHubWebhookService {
	secret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	return &GitHubWebhookService{
		Secret: secret,
	}
}

// GitHubWebhookResult represents the result of processing a webhook event
type GitHubWebhookResult struct {
	Event   string `json:"event"`
	Action  string `json:"action,omitempty"`
	Repo    string `json:"repo"`
	Summary string `json:"summary"`
}

// --- Payload types ---

// GitHubUser represents a GitHub user in webhook payloads
type GitHubUser struct {
	Login string `json:"login"`
}

// GitHubRepo represents a GitHub repository in webhook payloads
type GitHubRepo struct {
	FullName string `json:"full_name"`
	Name     string `json:"name"`
}

// GitHubCommit represents a commit in a push event
type GitHubCommit struct {
	ID       string `json:"id"`
	Message  string `json:"message"`
	Author   struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"author"`
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Removed  []string `json:"removed"`
}

// GitHubPushPayload represents a push event payload
type GitHubPushPayload struct {
	Ref        string         `json:"ref"`
	Before     string         `json:"before"`
	After      string         `json:"after"`
	Commits    []GitHubCommit `json:"commits"`
	Repository GitHubRepo     `json:"repository"`
	Sender     GitHubUser     `json:"sender"`
	Forced     bool           `json:"forced"`
}

// GitHubPullRequest represents a PR in webhook payloads
type GitHubPullRequest struct {
	Number       int        `json:"number"`
	Title        string     `json:"title"`
	State        string     `json:"state"`
	HTMLURL      string     `json:"html_url"`
	User         GitHubUser `json:"user"`
	Head         struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
	Additions    int  `json:"additions"`
	Deletions    int  `json:"deletions"`
	ChangedFiles int  `json:"changed_files"`
	Merged       bool `json:"merged"`
}

// GitHubPRPayload represents a pull_request event payload
type GitHubPRPayload struct {
	Action      string            `json:"action"`
	Number      int               `json:"number"`
	PullRequest GitHubPullRequest `json:"pull_request"`
	Repository  GitHubRepo        `json:"repository"`
	Sender      GitHubUser        `json:"sender"`
}

// GitHubLabel represents a label in webhook payloads
type GitHubLabel struct {
	Name string `json:"name"`
}

// GitHubIssue represents an issue in webhook payloads
type GitHubIssue struct {
	Number  int           `json:"number"`
	Title   string        `json:"title"`
	State   string        `json:"state"`
	HTMLURL string        `json:"html_url"`
	User    GitHubUser    `json:"user"`
	Body    string        `json:"body"`
	Labels  []GitHubLabel `json:"labels"`
}

// GitHubIssuePayload represents an issues event payload
type GitHubIssuePayload struct {
	Action     string      `json:"action"`
	Issue      GitHubIssue `json:"issue"`
	Repository GitHubRepo  `json:"repository"`
	Sender     GitHubUser  `json:"sender"`
}

// GitHubComment represents a comment in webhook payloads
type GitHubComment struct {
	ID      int        `json:"id"`
	Body    string     `json:"body"`
	User    GitHubUser `json:"user"`
	HTMLURL string     `json:"html_url"`
}

// GitHubIssueCommentPayload represents an issue_comment event payload
type GitHubIssueCommentPayload struct {
	Action     string        `json:"action"`
	Issue      GitHubIssue   `json:"issue"`
	Comment    GitHubComment `json:"comment"`
	Repository GitHubRepo    `json:"repository"`
	Sender     GitHubUser    `json:"sender"`
}

// GitHubReview represents a PR review in webhook payloads
type GitHubReview struct {
	ID      int        `json:"id"`
	State   string     `json:"state"` // approved, changes_requested, commented
	Body    string     `json:"body"`
	User    GitHubUser `json:"user"`
	HTMLURL string     `json:"html_url"`
}

// GitHubPRReviewPayload represents a pull_request_review event payload
type GitHubPRReviewPayload struct {
	Action      string            `json:"action"`
	Review      GitHubReview      `json:"review"`
	PullRequest GitHubPullRequest `json:"pull_request"`
	Repository  GitHubRepo        `json:"repository"`
	Sender      GitHubUser        `json:"sender"`
}

// GitHubPingPayload represents a ping event payload
type GitHubPingPayload struct {
	Zen        string     `json:"zen"`
	HookID     int        `json:"hook_id"`
	Repository GitHubRepo `json:"repository"`
}

// --- Signature verification ---

// VerifySignature verifies the HMAC-SHA256 signature of a GitHub webhook payload.
// Returns the raw body and whether the signature is valid.
func (s *GitHubWebhookService) VerifySignature(r *http.Request) ([]byte, bool) {
	signature := r.Header.Get("X-Hub-Signature-256")
	if s.Secret == "" {
		// If no secret configured, read body but skip verification
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, false
		}
		log.Println("[WARN] GITHUB_WEBHOOK_SECRET not configured, skipping signature verification")
		return body, true
	}

	if signature == "" {
		return nil, false
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, false
	}

	h := hmac.New(sha256.New, []byte(s.Secret))
	h.Write(body)
	expected := "sha256=" + hex.EncodeToString(h.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	return body, hmac.Equal([]byte(signature), []byte(expected))
}

// --- Event handlers ---

// HandleEvent processes a GitHub webhook event and returns the result
func (s *GitHubWebhookService) HandleEvent(event string, body []byte) (*GitHubWebhookResult, error) {
	switch event {
	case "ping":
		return s.handlePing(body)
	case "push":
		return s.handlePush(body)
	case "pull_request":
		return s.handlePullRequest(body)
	case "issues":
		return s.handleIssues(body)
	case "issue_comment":
		return s.handleIssueComment(body)
	case "pull_request_review":
		return s.handlePullRequestReview(body)
	default:
		log.Printf("[GitHub Webhook] Unhandled event type: %s", event)
		return &GitHubWebhookResult{
			Event:   event,
			Summary: fmt.Sprintf("Received unhandled event: %s", event),
		}, nil
	}
}

func (s *GitHubWebhookService) handlePing(body []byte) (*GitHubWebhookResult, error) {
	var payload GitHubPingPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse ping payload: %w", err)
	}

	log.Printf("[GitHub Webhook] Ping received: %s (hook_id: %d)", payload.Zen, payload.HookID)

	return &GitHubWebhookResult{
		Event:   "ping",
		Repo:    payload.Repository.FullName,
		Summary: fmt.Sprintf("Webhook configured: %s", payload.Zen),
	}, nil
}

func (s *GitHubWebhookService) handlePush(body []byte) (*GitHubWebhookResult, error) {
	var payload GitHubPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse push payload: %w", err)
	}

	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	commitCount := len(payload.Commits)

	summary := fmt.Sprintf("[%s] %s pushed %d commit(s) to %s",
		payload.Repository.FullName,
		payload.Sender.Login,
		commitCount,
		branch,
	)

	// Build commit messages summary
	var commitMsgs []string
	for _, c := range payload.Commits {
		msg := c.Message
		if idx := strings.Index(msg, "\n"); idx > 0 {
			msg = msg[:idx]
		}
		commitMsgs = append(commitMsgs, fmt.Sprintf("- `%s` %s", c.ID[:7], msg))
	}
	if len(commitMsgs) > 0 {
		summary += "\n" + strings.Join(commitMsgs, "\n")
	}

	log.Printf("[GitHub Webhook] Push: %s → %s (%d commits)", payload.Repository.FullName, branch, commitCount)

	// Trigger auto-upgrade for claude-agent-hub pushes to main
	if payload.Repository.Name == "claude-agent-hub" && branch == "main" {
		s.autoUpgradeMu.Lock()
		elapsed := time.Since(s.lastAutoUpgradeAt)
		s.autoUpgradeMu.Unlock()

		if elapsed >= 5*time.Minute {
			s.autoUpgradeMu.Lock()
			s.lastAutoUpgradeAt = time.Now()
			s.autoUpgradeMu.Unlock()

			go s.triggerAutoUpgrade(payload)
		} else {
			log.Printf("[GitHub Webhook] Auto-upgrade skipped (debounce: %v since last run)", elapsed)
		}
	}

	return &GitHubWebhookResult{
		Event:   "push",
		Repo:    payload.Repository.FullName,
		Summary: summary,
	}, nil
}

func (s *GitHubWebhookService) handlePullRequest(body []byte) (*GitHubWebhookResult, error) {
	var payload GitHubPRPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse pull_request payload: %w", err)
	}

	pr := payload.PullRequest
	summary := fmt.Sprintf("[%s] PR #%d %s: %s\nBy: %s | %s → %s\n+%d -%d (%d files)\n%s",
		payload.Repository.FullName,
		pr.Number,
		payload.Action,
		pr.Title,
		pr.User.Login,
		pr.Head.Ref,
		pr.Base.Ref,
		pr.Additions,
		pr.Deletions,
		pr.ChangedFiles,
		pr.HTMLURL,
	)

	log.Printf("[GitHub Webhook] PR #%d %s: %s (%s)", pr.Number, payload.Action, pr.Title, payload.Repository.FullName)

	return &GitHubWebhookResult{
		Event:   "pull_request",
		Action:  payload.Action,
		Repo:    payload.Repository.FullName,
		Summary: summary,
	}, nil
}

func (s *GitHubWebhookService) handleIssues(body []byte) (*GitHubWebhookResult, error) {
	var payload GitHubIssuePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse issues payload: %w", err)
	}

	issue := payload.Issue
	summary := fmt.Sprintf("[%s] Issue #%d %s: %s\nBy: %s\n%s",
		payload.Repository.FullName,
		issue.Number,
		payload.Action,
		issue.Title,
		issue.User.Login,
		issue.HTMLURL,
	)

	log.Printf("[GitHub Webhook] Issue #%d %s: %s (%s)", issue.Number, payload.Action, issue.Title, payload.Repository.FullName)

	// Epic phase auto-promotion: when a phase issue is closed, promote the next phase
	if payload.Action == "closed" {
		go s.promoteEpicPhase(payload.Repository.FullName, issue.Number)
	}

	return &GitHubWebhookResult{
		Event:   "issues",
		Action:  payload.Action,
		Repo:    payload.Repository.FullName,
		Summary: summary,
	}, nil
}

func (s *GitHubWebhookService) handleIssueComment(body []byte) (*GitHubWebhookResult, error) {
	var payload GitHubIssueCommentPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse issue_comment payload: %w", err)
	}

	issue := payload.Issue
	comment := payload.Comment

	// Truncate long comment bodies
	commentBody := comment.Body
	if len(commentBody) > 200 {
		commentBody = commentBody[:200] + "..."
	}

	summary := fmt.Sprintf("[%s] Comment on #%d (%s)\nBy: %s\n%s\n%s",
		payload.Repository.FullName,
		issue.Number,
		issue.Title,
		comment.User.Login,
		commentBody,
		comment.HTMLURL,
	)

	log.Printf("[GitHub Webhook] Comment on #%d by %s (%s)", issue.Number, comment.User.Login, payload.Repository.FullName)

	return &GitHubWebhookResult{
		Event:   "issue_comment",
		Action:  payload.Action,
		Repo:    payload.Repository.FullName,
		Summary: summary,
	}, nil
}

func (s *GitHubWebhookService) handlePullRequestReview(body []byte) (*GitHubWebhookResult, error) {
	var payload GitHubPRReviewPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse pull_request_review payload: %w", err)
	}

	review := payload.Review
	pr := payload.PullRequest

	stateLabel := review.State
	switch review.State {
	case "approved":
		stateLabel = "approved"
	case "changes_requested":
		stateLabel = "changes requested"
	case "commented":
		stateLabel = "reviewed"
	}

	summary := fmt.Sprintf("[%s] PR #%d %s by %s\n%s\n%s",
		payload.Repository.FullName,
		pr.Number,
		stateLabel,
		review.User.Login,
		pr.Title,
		pr.HTMLURL,
	)

	if review.Body != "" {
		reviewBody := review.Body
		if len(reviewBody) > 200 {
			reviewBody = reviewBody[:200] + "..."
		}
		summary += "\n" + reviewBody
	}

	log.Printf("[GitHub Webhook] PR #%d %s by %s (%s)", pr.Number, stateLabel, review.User.Login, payload.Repository.FullName)

	return &GitHubWebhookResult{
		Event:   "pull_request_review",
		Action:  payload.Action,
		Repo:    payload.Repository.FullName,
		Summary: summary,
	}, nil
}

// --- Auto-upgrade ---

// triggerAutoUpgrade detects changed layers from push commits and runs selective upgrade.
func (s *GitHubWebhookService) triggerAutoUpgrade(payload GitHubPushPayload) {
	layers := s.detectChangedLayers(payload.Commits)
	if len(layers) == 0 {
		log.Println("[GitHub Webhook] Auto-upgrade: no upgradable layers detected, skipping")
		return
	}

	layer := "all"
	if len(layers) == 1 {
		layer = layers[0]
	}

	req := UpgradeRequest{
		Layer: layer,
		Force: true,
	}

	result, err := RunSelectiveUpgrade(projectDir(), req)
	if err != nil {
		msg := fmt.Sprintf("Layer: %s\nError: %s", layer, result.Error)
		if result.Output != "" && len(result.Output) <= 500 {
			msg += "\nOutput: " + result.Output
		}
		log.Printf("[GitHub Webhook] Auto-upgrade failed: %v", err)
	} else {
		msg := fmt.Sprintf("Layer: %s\nStatus: %s", layer, result.Status)
		if len(result.LayersUpdated) > 0 {
			msg += "\nUpdated: " + strings.Join(result.LayersUpdated, ", ")
		}
		log.Printf("[GitHub Webhook] Auto-upgrade completed: layer=%s status=%s", layer, result.Status)
	}
}

// detectChangedLayers analyzes commit file paths and returns affected upgrade layers.
func (s *GitHubWebhookService) detectChangedLayers(commits []GitHubCommit) []string {
	layerSet := make(map[string]bool)
	for _, c := range commits {
		files := make([]string, 0, len(c.Added)+len(c.Modified)+len(c.Removed))
		files = append(files, c.Added...)
		files = append(files, c.Modified...)
		files = append(files, c.Removed...)
		for _, f := range files {
			if strings.HasPrefix(f, "api/") {
				layerSet["api"] = true
			} else if strings.HasPrefix(f, "frontend/") {
				layerSet["ui"] = true
			} else if strings.HasPrefix(f, "agents/") || strings.HasPrefix(f, ".claude/") || strings.HasPrefix(f, "config/") || strings.HasPrefix(f, "scripts/hooks/") {
				layerSet["tmux"] = true
			}
		}
	}
	layers := make([]string, 0, len(layerSet))
	for l := range layerSet {
		layers = append(layers, l)
	}
	return layers
}

// projectDir returns the project root directory.
func projectDir() string {
	if dir := os.Getenv("PROJECT_DIR"); dir != "" {
		return dir
	}
	return "/home/hartdev/projects/claude-agent-hub"
}

// --- Epic phase promotion ---

func (s *GitHubWebhookService) promoteEpicPhase(repoFullName string, issueNumber int) {
	promoter := &EpicPhasePromoter{Repo: repoFullName}
	if err := promoter.PromoteNextPhase(issueNumber); err != nil {
		log.Printf("[GitHub Webhook] Epic phase promotion failed for #%d: %v", issueNumber, err)
	}
}
