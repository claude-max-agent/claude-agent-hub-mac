package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

type TmuxSession struct {
	Name        string `json:"name"`
	Created     int64  `json:"created"`
	Attached    bool   `json:"attached"`
	Status      string `json:"status"`
	Agent       string `json:"agent"`
	CliType     string `json:"cli_type"`
	Description string `json:"description,omitempty"`
	Template    string `json:"template,omitempty"`
	IssueNumber string `json:"issue_number,omitempty"`
	PoolStatus  string `json:"pool_status,omitempty"`
}

type PoolStatusResponse struct {
	Pooled   int `json:"pooled"`
	Running  int `json:"running"`
	Stopping int `json:"stopping"`
	Total    int `json:"total"`
}

type managedInstances struct {
	Instances map[string]managedInstance `json:"instances"`
}

type managedInstance struct {
	Session     string `json:"session"`
	Status      string `json:"status"`
	WorkingDir  string `json:"working_dir"`
	Template    string `json:"template"`
	IssueNumber string `json:"issue_number"`
	Description string `json:"description"`
	StartedAt   string `json:"started_at"`
	AssignedAt  string `json:"assigned_at"`
	CliCommand  string `json:"cli_command,omitempty"`
}

type CliType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DefaultArgs string `json:"default_args"`
}

type SessionHandler struct {
	mu       sync.RWMutex
	sessions map[string]*sessionMeta
}

type sessionMeta struct {
	Agent           string `json:"agent"`
	CliType         string `json:"cli_type"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	Created         int64  `json:"created"`
}

func NewSessionHandler() *SessionHandler {
	return &SessionHandler{
		sessions: make(map[string]*sessionMeta),
	}
}

func projectDir() string {
	dir := os.Getenv("CLAUDE_AGENT_HUB_DIR")
	if dir == "" {
		if exe, err := os.Executable(); err == nil {
			dir = filepath.Dir(exe)
		}
	}
	return dir
}

func readManagedInstances() (*managedInstances, error) {
	stateFile := filepath.Join(projectDir(), ".claude", "state", "managed-instances.json")
	f, err := os.Open(stateFile)
	if err != nil {
		return &managedInstances{Instances: map[string]managedInstance{}}, nil
	}
	defer f.Close()
	var mi managedInstances
	if err := json.NewDecoder(f).Decode(&mi); err != nil {
		return &managedInstances{Instances: map[string]managedInstance{}}, nil
	}
	return &mi, nil
}

func (h *SessionHandler) HandlePoolStatus(w http.ResponseWriter, r *http.Request) {
	mi, _ := readManagedInstances()
	var ps PoolStatusResponse
	for _, inst := range mi.Instances {
		switch inst.Status {
		case "pooled":
			ps.Pooled++
		case "running":
			ps.Running++
		case "stopping":
			ps.Stopping++
		}
		ps.Total++
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ps)
}

func (h *SessionHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	// Get tmux sessions
	sessions := h.listTmuxSessions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func (h *SessionHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string `json:"name"`
		Agent           string `json:"agent"`
		CliType         string `json:"cli_type"`
		Model           string `json:"model"`
		ReasoningEffort string `json:"reasoning_effort"`
		Description     string `json:"description"`
		IssueNumber     string `json:"issue_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate model name if provided (prevent injection)
	if req.Model != "" {
		validModel := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
		if !validModel.MatchString(req.Model) {
			http.Error(w, `{"error":"invalid model name"}`, http.StatusBadRequest)
			return
		}
	}

	// Validate reasoning effort
	if req.ReasoningEffort != "" {
		validEfforts := map[string]bool{"low": true, "medium": true, "high": true}
		if !validEfforts[req.ReasoningEffort] {
			http.Error(w, `{"error":"reasoning_effort must be low, medium, or high"}`, http.StatusBadRequest)
			return
		}
	}

	// Auto-detect CLI type from model name if not explicitly provided
	if req.CliType == "" {
		if strings.HasPrefix(req.Model, "gpt-") || strings.HasPrefix(req.Model, "o1-") || strings.HasPrefix(req.Model, "o3-") || strings.HasPrefix(req.Model, "o4-") {
			req.CliType = "codex"
		} else {
			req.CliType = "claude"
		}
	}

	if req.CliType != "claude" && req.CliType != "codex" {
		http.Error(w, `{"error":"unsupported cli_type"}`, http.StatusBadRequest)
		return
	}

	// Read managed-instances.json to find a pooled slot
	mi, err := readManagedInstances()
	if err != nil {
		http.Error(w, `{"error":"failed to read pool state"}`, http.StatusInternalServerError)
		return
	}

	// Find an available pooled slot (auto-select regardless of custom name)
	var slotName string
	{
		var keys []string
		for name := range mi.Instances {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			// pool-0 is reserved for patrol (see config/services.yaml)
			if name == "pool-0" {
				continue
			}
			if mi.Instances[name].Status == "pooled" {
				slotName = name
				break
			}
		}
	}

	// If custom name was provided, use it as description (alias)
	if req.Name != "" && req.Description == "" {
		req.Description = req.Name
	}
	if slotName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "利用可能なスロットがありません（プール満タン）"})
		return
	}

	// Determine template from agent name
	template := "dev"
	if req.Agent != "" {
		template = req.Agent
	}

	// Build CLI options from model/reasoning_effort
	var cliOpts []string
	if req.Model != "" {
		cliOpts = append(cliOpts, "--model", req.Model)
	}
	if req.ReasoningEffort != "" {
		cliOpts = append(cliOpts, "--reasoning-effort", req.ReasoningEffort)
	}

	// Run instance-manager.sh assign
	pDir := projectDir()
	scriptPath := filepath.Join(pDir, "scripts", "instance-manager.sh")
	assignArgs := []string{scriptPath, "assign", slotName, "--template", template}
	if len(cliOpts) > 0 {
		assignArgs = append(assignArgs, "--cli-opts", strings.Join(cliOpts, " "))
	}
	if req.CliType == "codex" {
		assignArgs = append(assignArgs, "--cli-type", "codex")
	}
	if req.Description != "" {
		assignArgs = append(assignArgs, "--description", req.Description)
	}
	if req.IssueNumber != "" {
		assignArgs = append(assignArgs, "--issue", req.IssueNumber)
	}
	assignCmd := exec.Command("bash", assignArgs...)
	assignCmd.Dir = pDir
	assignCmd.Env = os.Environ()
	output, err := assignCmd.CombinedOutput()
	if err != nil {
		log.Printf("instance-manager.sh assign failed for %s: %v\nOutput: %s", slotName, err, string(output))
		http.Error(w, fmt.Sprintf(`{"error":"スロット割り当て失敗: %s"}`, strings.TrimSpace(string(output))), http.StatusInternalServerError)
		return
	}

	// Store metadata
	h.mu.Lock()
	h.sessions[slotName] = &sessionMeta{
		Agent:           req.Agent,
		CliType:         req.CliType,
		Model:           req.Model,
		ReasoningEffort: req.ReasoningEffort,
		Created:         time.Now().Unix(),
	}
	h.mu.Unlock()

	log.Printf("Pool session assigned: %s (cli=%s, model=%s, agent=%s, desc=%s, issue=%s)", slotName, req.CliType, req.Model, req.Agent, req.Description, req.IssueNumber)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"session": slotName,
	})
}

func (h *SessionHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "sessionName")
	if name == "" {
		http.Error(w, `{"error":"session name required"}`, http.StatusBadRequest)
		return
	}

	// Use instance-manager.sh release to properly return the pool slot
	pDir := projectDir()
	scriptPath := filepath.Join(pDir, "scripts", "instance-manager.sh")
	releaseCmd := exec.Command("bash", scriptPath, "release", name)
	releaseCmd.Dir = pDir
	releaseCmd.Env = os.Environ()
	output, err := releaseCmd.CombinedOutput()
	if err != nil {
		log.Printf("instance-manager.sh release failed for %s: %v\nOutput: %s", name, err, string(output))
		// Fallback: try tmux kill-session directly
		killCmd := exec.Command("tmux", "kill-session", "-t", name)
		if killErr := killCmd.Run(); killErr != nil {
			log.Printf("Fallback tmux kill-session also failed for %s: %v", name, killErr)
			http.Error(w, fmt.Sprintf(`{"error":"failed to stop session: %v"}`, err), http.StatusInternalServerError)
			return
		}
	}

	h.mu.Lock()
	delete(h.sessions, name)
	h.mu.Unlock()

	log.Printf("Session released: %s", name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"session": name,
	})
}

func (h *SessionHandler) HandleRestart(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "sessionName")
	if name == "" {
		http.Error(w, `{"error":"session name required"}`, http.StatusBadRequest)
		return
	}

	// Validate session name (prevent injection)
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(name) {
		http.Error(w, `{"error":"invalid session name"}`, http.StatusBadRequest)
		return
	}

	if name == "manager" {
		if err := restartManagerSession(); err != nil {
			log.Printf("manager restart failed: %v", err)
			http.Error(w, fmt.Sprintf(`{"error":"failed to restart manager session: %v"}`, err), http.StatusInternalServerError)
			return
		}

		log.Printf("Session restarted: %s", name)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"session": name,
		})
		return
	}

	// Read managed-instances.json to get cli_command
	mi, err := readManagedInstances()
	if err != nil {
		http.Error(w, `{"error":"failed to read managed instances"}`, http.StatusInternalServerError)
		return
	}

	inst, ok := mi.Instances[name]
	if !ok {
		http.Error(w, `{"error":"session not found in managed instances"}`, http.StatusNotFound)
		return
	}

	cliCommand := inst.CliCommand
	if cliCommand == "" {
		http.Error(w, `{"error":"session has no stored cli_command"}`, http.StatusConflict)
		return
	}

	// Check that the tmux session exists
	checkCmd := exec.Command("tmux", "has-session", "-t", name)
	sessionExists := checkCmd.Run() == nil

	if sessionExists {
		// Step 1: Send /exit to the tmux session to gracefully stop Claude
		exitCmd := exec.Command("tmux", "send-keys", "-t", name, "/exit", "Enter")
		if exitErr := exitCmd.Run(); exitErr != nil {
			log.Printf("Failed to send /exit to %s: %v", name, exitErr)
			http.Error(w, `{"error":"failed to send /exit to session"}`, http.StatusInternalServerError)
			return
		}

		// Step 2: Wait briefly for the process to exit
		time.Sleep(3 * time.Second)

		// Step 3: If /exit didn't work, send Ctrl+C to terminate the process tree
		checkCmd2 := exec.Command("tmux", "has-session", "-t", name)
		if checkCmd2.Run() == nil {
			ctrlcCmd := exec.Command("tmux", "send-keys", "-t", name, "C-c")
			if ctrlcErr := ctrlcCmd.Run(); ctrlcErr != nil {
				log.Printf("Failed to send C-c to %s: %v", name, ctrlcErr)
			}
			time.Sleep(1 * time.Second)
		}
	}

	// Step 4: Re-launch the CLI command in the tmux session
	if !sessionExists {
		sessionDir := filepath.Join(projectDir(), inst.WorkingDir)
		if err := os.MkdirAll(sessionDir, 0o755); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to prepare session dir: %v"}`, err), http.StatusInternalServerError)
			return
		}

		newSessionCmd := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", sessionDir, "-x", "200", "-y", "50")
		newSessionCmd.Env = os.Environ()
		if err := newSessionCmd.Run(); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to recreate tmux session: %v"}`, err), http.StatusInternalServerError)
			return
		}
	}

	launchCmd := exec.Command("tmux", "send-keys", "-t", name, cliCommand, "Enter")
	if launchErr := launchCmd.Run(); launchErr != nil {
		log.Printf("Failed to relaunch CLI in %s: %v", name, launchErr)
		http.Error(w, `{"error":"failed to relaunch CLI in session"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("Session restarted: %s (cmd=%s)", name, cliCommand)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"session": name,
	})
}

func restartManagerSession() error {
	if exec.Command("tmux", "has-session", "-t", "manager").Run() == nil {
		exitCmd := exec.Command("tmux", "send-keys", "-t", "manager", "/exit", "Enter")
		if err := exitCmd.Run(); err != nil {
			return fmt.Errorf("failed to send /exit to manager session: %w", err)
		}

		time.Sleep(3 * time.Second)

		if exec.Command("tmux", "has-session", "-t", "manager").Run() == nil {
			if err := exec.Command("tmux", "send-keys", "-t", "manager", "C-c").Run(); err != nil {
				log.Printf("Failed to send C-c to manager: %v", err)
			}
			time.Sleep(1 * time.Second)
		}
	}

	sessionDir := filepath.Join(projectDir(), "agents", "manager")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return fmt.Errorf("failed to prepare manager session dir: %w", err)
	}

	if err := exec.Command("tmux", "kill-session", "-t", "manager").Run(); err != nil {
		// Ignore errors here: the session may already have exited after /exit.
		log.Printf("Failed to kill existing manager session: %v", err)
	}

	if err := exec.Command("tmux", "new-session", "-d", "-s", "manager", "-c", sessionDir, "-x", "200", "-y", "50").Run(); err != nil {
		return fmt.Errorf("failed to create manager tmux session: %w", err)
	}
	if err := exec.Command("tmux", "rename-window", "-t", "manager:0", "manager").Run(); err != nil {
		return fmt.Errorf("failed to rename manager tmux window: %w", err)
	}

	claudeCmd := "unset CLAUDECODE && claude --dangerously-skip-permissions --effort high"
	initMessage := "Create an Agent Team with 1 teammates and stand by."
	sendCmd := exec.Command(
		"tmux",
		"send-keys",
		"-t",
		"manager:0",
		fmt.Sprintf("cd '%s' && %s '%s'", sessionDir, claudeCmd, initMessage),
		"C-m",
	)
	sendCmd.Env = os.Environ()
	if err := sendCmd.Run(); err != nil {
		return fmt.Errorf("failed to launch manager Claude session: %w", err)
	}

	return nil
}

func (h *SessionHandler) HandleUpdateDescription(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "sessionName")
	if name == "" {
		http.Error(w, `{"error":"session name required"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	pDir := projectDir()
	stateFile := filepath.Join(pDir, ".claude", "state", "managed-instances.json")
	lockFile := stateFile + ".lock"
	descCmd := exec.Command("bash", "-c",
		fmt.Sprintf(`flock -w 10 %s jq --arg name %q --arg desc %q '.instances[$name].description = $desc' %s > %s.tmp && mv %s.tmp %s`,
			lockFile, name, req.Description, stateFile, stateFile, stateFile, stateFile))
	descCmd.Dir = pDir
	if err := descCmd.Run(); err != nil {
		log.Printf("Failed to update description for %s: %v", name, err)
		http.Error(w, `{"error":"failed to update description"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("Description updated: %s = %q", name, req.Description)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"session": name,
	})
}

func (h *SessionHandler) HandleListAgents(w http.ResponseWriter, r *http.Request) {
	type agentInfo struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	seen := make(map[string]bool)
	var agents []agentInfo

	// Scan multiple .claude/agents/ directories (project-local first, then home)
	homeDir, _ := os.UserHomeDir()
	projectDir := os.Getenv("CLAUDE_AGENT_HUB_DIR")
	if projectDir == "" {
		projectDir = os.Getenv("PROJECT_DIR")
	}
	if projectDir == "" {
		// Fallback: derive from the running binary's location
		// The binary is at api/cmd/server/main.go -> built to api/cmd/server/server
		// So project root is 3 levels up from the binary
		if exe, err := os.Executable(); err == nil {
			resolved, err := filepath.EvalSymlinks(exe)
			if err == nil {
				exe = resolved
			}
			candidate := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
			// Verify this looks like the project root by checking for .claude/agents/
			if _, err := os.Stat(filepath.Join(candidate, ".claude", "agents")); err == nil {
				projectDir = candidate
			}
		}
	}
	if projectDir == "" && homeDir != "" {
		// Last resort: check known project path
		candidate := filepath.Join(homeDir, "projects", "claude-agent-hub")
		if _, err := os.Stat(filepath.Join(candidate, ".claude", "agents")); err == nil {
			projectDir = candidate
		}
	}

	var dirs []string
	if projectDir != "" {
		dirs = append(dirs, filepath.Join(projectDir, ".claude", "agents"))
	}
	dirs = append(dirs, filepath.Join(homeDir, ".claude", "agents"))

	for _, agentsDir := range dirs {
		entries, err := os.ReadDir(agentsDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			aType := "file"
			if e.IsDir() {
				aType = "directory"
			} else {
				if !strings.HasSuffix(name, ".md") {
					continue
				}
				name = strings.TrimSuffix(name, ".md")
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			agents = append(agents, agentInfo{Name: name, Type: aType})
		}
	}

	if agents == nil {
		agents = []agentInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

func (h *SessionHandler) HandleListCliTypes(w http.ResponseWriter, r *http.Request) {
	cliTypes := []CliType{
		{ID: "claude", Name: "Claude Code", DefaultArgs: "--dangerously-skip-permissions"},
		{ID: "codex", Name: "Codex CLI", DefaultArgs: "-a never -s danger-full-access"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cliTypes)
}

func (h *SessionHandler) listTmuxSessions() []TmuxSession {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}\t#{session_created}\t#{session_attached}")
	out, err := cmd.Output()
	if err != nil {
		return []TmuxSession{}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	// Read managed-instances for pool metadata
	mi, _ := readManagedInstances()

	var sessions []TmuxSession
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}

		created, _ := strconv.ParseInt(parts[1], 10, 64)
		attached := parts[2] == "1"

		s := TmuxSession{
			Name:     parts[0],
			Created:  created,
			Attached: attached,
			Status:   "running",
		}

		// Enrich with in-memory metadata
		if meta, ok := h.sessions[s.Name]; ok {
			s.Agent = meta.Agent
			s.CliType = meta.CliType
		}

		// Enrich with managed-instances metadata
		if inst, ok := mi.Instances[s.Name]; ok {
			s.Description = inst.Description
			s.Template = inst.Template
			s.IssueNumber = inst.IssueNumber
			s.PoolStatus = inst.Status
		}

		sessions = append(sessions, s)
	}

	if sessions == nil {
		sessions = []TmuxSession{}
	}
	return sessions
}
