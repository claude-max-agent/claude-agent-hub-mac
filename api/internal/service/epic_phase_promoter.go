package service

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// GhCommandFunc is a function type for running gh CLI commands.
type GhCommandFunc func(args ...string) (string, error)

// EpicPhasePromoter handles automatic promotion of epic phase issues.
// When a phase issue is closed, it finds the parent epic and adds "ready"
// label to the next uncompleted phase. If all phases are done, it closes the epic.
type EpicPhasePromoter struct {
	Repo      string        // e.g. "claude-max-agent/claude-agent-hub"
	RunGhCmd  GhCommandFunc // injectable for testing; defaults to real gh CLI
}

// phaseIssue represents a child issue referenced in an epic body
type phaseIssue struct {
	Number    int
	Completed bool
}

// phaseRefPattern matches checklist items like "- [ ] #101" or "- [x] #101"
// Also supports URLs like "- [ ] https://github.com/owner/repo/issues/101"
var phaseRefPattern = regexp.MustCompile(`- \[([ xX])\] (?:#(\d+)|https?://github\.com/[^/]+/[^/]+/issues/(\d+))`)

// epicLabelName is the label used to identify epic issues
const epicLabelName = "epic"

func (p *EpicPhasePromoter) gh(args ...string) (string, error) {
	if p.RunGhCmd != nil {
		return p.RunGhCmd(args...)
	}
	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// PromoteNextPhase checks if the closed issue belongs to an epic,
// and if so, promotes the next phase by adding the "ready" label.
func (p *EpicPhasePromoter) PromoteNextPhase(closedIssueNumber int) error {
	epicNumber, err := p.findParentEpic(closedIssueNumber)
	if err != nil {
		return fmt.Errorf("finding parent epic: %w", err)
	}
	if epicNumber == 0 {
		log.Printf("[EpicPhasePromoter] Issue #%d is not part of any epic, skipping", closedIssueNumber)
		return nil
	}

	log.Printf("[EpicPhasePromoter] Issue #%d belongs to epic #%d", closedIssueNumber, epicNumber)

	epicBody, err := p.getIssueBody(epicNumber)
	if err != nil {
		return fmt.Errorf("getting epic #%d body: %w", epicNumber, err)
	}

	phases := parsePhaseList(epicBody)
	if len(phases) == 0 {
		log.Printf("[EpicPhasePromoter] Epic #%d has no phase checklist, skipping", epicNumber)
		return nil
	}

	// Find next uncompleted phase (skip the one just closed)
	var nextPhase int
	allDone := true
	for _, ph := range phases {
		if ph.Number == closedIssueNumber {
			continue
		}
		if !ph.Completed {
			if nextPhase == 0 {
				nextPhase = ph.Number
			}
			allDone = false
		}
	}

	if allDone {
		log.Printf("[EpicPhasePromoter] All phases of epic #%d are completed, closing epic", epicNumber)
		if err := p.closeEpic(epicNumber); err != nil {
			return fmt.Errorf("closing epic #%d: %w", epicNumber, err)
		}
		return nil
	}

	if nextPhase > 0 {
		log.Printf("[EpicPhasePromoter] Promoting issue #%d to ready (epic #%d)", nextPhase, epicNumber)
		if err := p.addReadyLabel(nextPhase); err != nil {
			return fmt.Errorf("adding ready label to #%d: %w", nextPhase, err)
		}
	}

	return nil
}

func (p *EpicPhasePromoter) findParentEpic(issueNumber int) (int, error) {
	query := fmt.Sprintf("label:%s state:open #%d in:body", epicLabelName, issueNumber)
	out, err := p.gh("issue", "list",
		"--repo", p.Repo,
		"--search", query,
		"--json", "number",
		"--jq", ".[].number",
	)
	if err != nil {
		return 0, fmt.Errorf("searching epics: %w (output: %s)", err, out)
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return 0, nil
	}

	lines := strings.Split(out, "\n")
	num, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, fmt.Errorf("parsing epic number %q: %w", lines[0], err)
	}
	return num, nil
}

func (p *EpicPhasePromoter) getIssueBody(issueNumber int) (string, error) {
	out, err := p.gh("issue", "view",
		"--repo", p.Repo,
		strconv.Itoa(issueNumber),
		"--json", "body",
		"--jq", ".body",
	)
	if err != nil {
		return "", fmt.Errorf("viewing issue #%d: %w (output: %s)", issueNumber, err, out)
	}
	return out, nil
}

// parsePhaseList extracts phase issue references from an epic body.
func parsePhaseList(body string) []phaseIssue {
	matches := phaseRefPattern.FindAllStringSubmatch(body, -1)
	var phases []phaseIssue
	for _, m := range matches {
		completed := strings.ToLower(m[1]) == "x"
		numStr := m[2]
		if numStr == "" {
			numStr = m[3] // URL format
		}
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		phases = append(phases, phaseIssue{Number: num, Completed: completed})
	}
	return phases
}

func (p *EpicPhasePromoter) addReadyLabel(issueNumber int) error {
	out, err := p.gh("issue", "edit",
		"--repo", p.Repo,
		strconv.Itoa(issueNumber),
		"--add-label", "ready",
		"--remove-label", "in-progress",
	)
	if err != nil {
		return fmt.Errorf("editing issue #%d: %w (output: %s)", issueNumber, err, out)
	}
	return nil
}

func (p *EpicPhasePromoter) closeEpic(epicNumber int) error {
	out, err := p.gh("issue", "close",
		"--repo", p.Repo,
		strconv.Itoa(epicNumber),
		"--comment", "All phases completed. Auto-closing epic.",
	)
	if err != nil {
		return fmt.Errorf("closing epic #%d: %w (output: %s)", epicNumber, err, out)
	}
	return nil
}
