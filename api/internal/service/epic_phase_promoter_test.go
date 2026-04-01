package service

import (
	"fmt"
	"strings"
	"testing"
)

func TestParsePhaseList(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected []phaseIssue
	}{
		{
			name: "standard checklist with hash references",
			body: `## Phases
- [x] #101 Phase 1: Data collection
- [ ] #102 Phase 2: API integration
- [ ] #103 Phase 3: Testing`,
			expected: []phaseIssue{
				{Number: 101, Completed: true},
				{Number: 102, Completed: false},
				{Number: 103, Completed: false},
			},
		},
		{
			name: "URL format references",
			body: `## Phases
- [x] https://github.com/claude-max-agent/claude-agent-hub/issues/101 Phase 1
- [ ] https://github.com/claude-max-agent/claude-agent-hub/issues/102 Phase 2`,
			expected: []phaseIssue{
				{Number: 101, Completed: true},
				{Number: 102, Completed: false},
			},
		},
		{
			name: "mixed format",
			body: `- [X] #50 Done
- [ ] #51 Next`,
			expected: []phaseIssue{
				{Number: 50, Completed: true},
				{Number: 51, Completed: false},
			},
		},
		{
			name: "no checklist",
			body: "This is a regular issue with no phases.",
			expected: nil,
		},
		{
			name: "all completed",
			body: `- [x] #10 Phase 1
- [x] #11 Phase 2
- [x] #12 Phase 3`,
			expected: []phaseIssue{
				{Number: 10, Completed: true},
				{Number: 11, Completed: true},
				{Number: 12, Completed: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePhaseList(tt.body)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d phases, got %d: %+v", len(tt.expected), len(result), result)
			}
			for i, exp := range tt.expected {
				if result[i].Number != exp.Number {
					t.Errorf("phase[%d].Number = %d, want %d", i, result[i].Number, exp.Number)
				}
				if result[i].Completed != exp.Completed {
					t.Errorf("phase[%d].Completed = %v, want %v", i, result[i].Completed, exp.Completed)
				}
			}
		})
	}
}

// mockGhCommand creates a mock gh command function that records calls and returns scripted responses.
func mockGhCommand(responses map[string]struct {
	output string
	err    error
}) (GhCommandFunc, *[][]string) {
	var calls [][]string
	fn := func(args ...string) (string, error) {
		calls = append(calls, args)
		key := strings.Join(args[:2], " ") // e.g. "issue list", "issue view"
		if resp, ok := responses[key]; ok {
			return resp.output, resp.err
		}
		return "", fmt.Errorf("unexpected gh command: %v", args)
	}
	return fn, &calls
}

func TestPromoteNextPhase_PromotesNext(t *testing.T) {
	ghFn, calls := mockGhCommand(map[string]struct {
		output string
		err    error
	}{
		"issue list": {output: "100\n", err: nil},
		"issue view": {
			output: "- [x] #201 Phase 1\n- [ ] #202 Phase 2\n- [ ] #203 Phase 3\n",
			err:    nil,
		},
		"issue edit": {output: "", err: nil},
	})

	p := &EpicPhasePromoter{
		Repo:     "claude-max-agent/claude-agent-hub",
		RunGhCmd: ghFn,
	}

	err := p.PromoteNextPhase(201)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have called: issue list (find epic), issue view (get body), issue edit (add ready label)
	if len(*calls) != 3 {
		t.Fatalf("expected 3 gh calls, got %d: %v", len(*calls), *calls)
	}

	// Verify the edit call targets issue #202
	editCall := (*calls)[2]
	if editCall[0] != "issue" || editCall[1] != "edit" {
		t.Errorf("expected issue edit, got: %v", editCall)
	}
	found202 := false
	for _, arg := range editCall {
		if arg == "202" {
			found202 = true
		}
	}
	if !found202 {
		t.Errorf("expected edit to target issue #202, got: %v", editCall)
	}
}

func TestPromoteNextPhase_AllDone_ClosesEpic(t *testing.T) {
	ghFn, calls := mockGhCommand(map[string]struct {
		output string
		err    error
	}{
		"issue list": {output: "100\n", err: nil},
		"issue view": {
			output: "- [x] #201 Phase 1\n- [x] #202 Phase 2\n- [x] #203 Phase 3\n",
			err:    nil,
		},
		"issue close": {output: "", err: nil},
	})

	p := &EpicPhasePromoter{
		Repo:     "claude-max-agent/claude-agent-hub",
		RunGhCmd: ghFn,
	}

	// Close the last remaining phase (203 was already marked [x] in body,
	// but we treat the closed issue as done regardless of checkbox state)
	err := p.PromoteNextPhase(203)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have called: issue list, issue view, issue close
	if len(*calls) != 3 {
		t.Fatalf("expected 3 gh calls, got %d: %v", len(*calls), *calls)
	}

	closeCall := (*calls)[2]
	if closeCall[0] != "issue" || closeCall[1] != "close" {
		t.Errorf("expected issue close, got: %v", closeCall)
	}
}

func TestPromoteNextPhase_NoEpic(t *testing.T) {
	ghFn, calls := mockGhCommand(map[string]struct {
		output string
		err    error
	}{
		"issue list": {output: "", err: nil},
	})

	p := &EpicPhasePromoter{
		Repo:     "claude-max-agent/claude-agent-hub",
		RunGhCmd: ghFn,
	}

	err := p.PromoteNextPhase(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only one call (search), no further action
	if len(*calls) != 1 {
		t.Fatalf("expected 1 gh call, got %d: %v", len(*calls), *calls)
	}
}

func TestPromoteNextPhase_SkipsClosedIssueInPhaseList(t *testing.T) {
	// Phase 1 (#201) is being closed now, but body still shows it as [ ] (unchecked).
	// The promoter should skip #201 and promote #202.
	ghFn, calls := mockGhCommand(map[string]struct {
		output string
		err    error
	}{
		"issue list": {output: "100\n", err: nil},
		"issue view": {
			output: "- [ ] #201 Phase 1\n- [ ] #202 Phase 2\n- [ ] #203 Phase 3\n",
			err:    nil,
		},
		"issue edit": {output: "", err: nil},
	})

	p := &EpicPhasePromoter{
		Repo:     "claude-max-agent/claude-agent-hub",
		RunGhCmd: ghFn,
	}

	err := p.PromoteNextPhase(201)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(*calls) != 3 {
		t.Fatalf("expected 3 gh calls, got %d: %v", len(*calls), *calls)
	}

	// Verify edit targets #202, not #201
	editCall := (*calls)[2]
	found202 := false
	for _, arg := range editCall {
		if arg == "202" {
			found202 = true
		}
	}
	if !found202 {
		t.Errorf("expected edit to target issue #202, got: %v", editCall)
	}
}
