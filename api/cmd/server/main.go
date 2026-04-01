package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/zono819/claude-agent-hub/api/internal/config"
	"github.com/zono819/claude-agent-hub/api/internal/database"
	"github.com/zono819/claude-agent-hub/api/internal/handler"
	"github.com/zono819/claude-agent-hub/api/internal/service"
)

// Build info - set via ldflags at build time
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

var (
	queueService         *service.QueueService
	requestService       *service.RequestService
	githubWebhookService *service.GitHubWebhookService
	githubAPIService     *service.GitHubAPIService

	// Database
	db           *database.DB
	taskRepo     *database.TaskRepository
	agentRepo    *database.AgentRepository
	requestRepo  *database.RequestRepository
	messageRepo  *database.MessageRepository
	questionRepo *database.QuestionRepository
	revenueRepo         *database.RevenueRepository
	cronJobRepo         *database.CronJobRepository

	// Configuration
	appConfig  *config.Config
	appManager *service.AppManager
)

// AgentInfo represents agent status info (moved from deleted discord_bot.go)
type AgentInfo struct {
	ID          string
	Role        string
	Status      string
	CurrentTask string
}

// AgentStatus represents the self-reported status of an agent
type AgentStatus struct {
	Status          string    `json:"status"`           // available, busy, stopped
	CurrentTask     string    `json:"current_task"`     // task ID if busy
	TaskDescription string    `json:"task_description"` // task description if busy
	LastHeartbeat   time.Time `json:"last_heartbeat"`   // last status report time
}

// In-memory agent status storage
var (
	agentStatuses   = make(map[string]*AgentStatus)
	agentStatusLock sync.RWMutex
	statusTimeout   = 10 * time.Minute // Agent considered stopped if no heartbeat for 10 minutes
)

// Manager activity tracking for idle auto-shutdown
var (
	managerLastActivity     = make(map[string]time.Time)
	managerLastActivityLock sync.RWMutex
)

// Manager auto-start mutex to prevent concurrent starts
var (
	managerAutoStartMu  sync.Mutex
	managerAutoStarting = make(map[string]bool)
)

func main() {
	// Load .env file if it exists (environment variables take precedence)
	// Try project root first, then current working directory
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or could not be loaded: %v", err)
	} else {
		log.Println(".env file loaded")
	}

	// Load configuration
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "config"
	}
	// Resolve relative configDir to absolute path based on executable directory
	if !filepath.IsAbs(configDir) {
		if exePath, err := os.Executable(); err == nil {
			resolved := filepath.Join(filepath.Dir(exePath), configDir)
			if _, err := os.Stat(resolved); err == nil {
				configDir = resolved
				log.Printf("Config directory resolved to: %s", configDir)
			} else {
				// Fall back to working directory relative path
				if absPath, err := filepath.Abs(configDir); err == nil {
					configDir = absPath
					log.Printf("Config directory resolved to: %s (working directory)", configDir)
				}
			}
		}
	}
	appConfig = config.LoadWithDefaults(configDir)
	log.Printf("Configuration loaded: %d workers enabled", appConfig.GetWorkerCount())

	// Initialize app manager for multi-repo management
	appsConfig := config.LoadAppsConfigWithDefaults(configDir)
	appManager = service.NewAppManager(appsConfig)
	log.Printf("App manager initialized: %d apps configured", len(appsConfig.Apps))

	// Initialize database using dynamic configuration
	// Priority: DATABASE_URL (PostgreSQL) > DB_TYPE env var > default (SQLite)
	dbConfig := database.ConfigFromEnv()
	log.Printf("Database configuration: type=%s", dbConfig.Type)

	var err error
	db, err = database.NewFromConfig(dbConfig)
	if err != nil {
		log.Printf("Warning: Failed to initialize database: %v (falling back to file-based storage)", err)
	} else {
		if err := db.Migrate(); err != nil {
			log.Printf("Warning: Failed to run migrations: %v", err)
		} else {
			log.Printf("Database initialized: type=%s", dbConfig.Type)
		}

		// Initialize repositories
		taskRepo = database.NewTaskRepository(db)
		agentRepo = database.NewAgentRepository(db)
		requestRepo = database.NewRequestRepository(db)
		messageRepo = database.NewMessageRepository(db)
		questionRepo = database.NewQuestionRepository(db)
		revenueRepo = database.NewRevenueRepository(db)
		cronJobRepo = database.NewCronJobRepository(db)
		log.Println("Database repositories initialized")
	}

	// Initialize services with database repositories
	if taskRepo != nil && messageRepo != nil {
		queueService = service.NewQueueService(taskRepo, messageRepo)
		log.Println("Queue service initialized with database")
	} else {
		log.Println("Warning: Queue service not initialized (database not available)")
	}

	if requestRepo != nil && taskRepo != nil {
		requestService = service.NewRequestService(requestRepo, taskRepo)
		log.Println("Request service initialized with database")
	} else {
		log.Println("Warning: Request service not initialized (database not available)")
	}

	// Initialize GitHub Webhook service
	githubWebhookService = service.NewGitHubWebhookService()
	if githubWebhookService.Secret != "" {
		log.Println("GitHub webhook secret configured")
	} else {
		log.Println("GitHub webhook secret not configured (signature verification disabled)")
	}

	// Initialize GitHub API service (for frontend proxy)
	var githubRepos []string
	for _, app := range appsConfig.Apps {
		if app.Repo != "" {
			githubRepos = append(githubRepos, app.Repo)
		}
	}
	githubAPIService = service.NewGitHubAPIService(githubRepos)
	if githubAPIService.IsConfigured() {
		log.Printf("GitHub API service configured: %s", githubAPIService.GetConfigStatus())
	} else {
		log.Println("GitHub API service not configured (gh command not authenticated and GITHUB_TOKEN not set)")
	}

	// Start monitoring services (Phase 7: idle shutdown disabled)
	if appConfig.Services != nil {
		go startIdleShutdownWatcher(nil)
		go startMemoryMonitor(appConfig.Services.MemoryMonitor)
		go startClaudeProcessHealthMonitor()
	}

	// Setup graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		if db != nil {
			db.Close()
			log.Println("Database closed")
		}
		os.Exit(0)
	}()

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// CORS for development
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Version
		r.Get("/version", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"version":    Version,
				"git_commit": GitCommit,
				"build_time": BuildTime,
				"component":  "api",
			})
		})

		// Tasks
		r.Route("/tasks", func(r chi.Router) {
			r.Get("/", listTasks)
			r.Post("/", createTask)
			r.Delete("/", cancelAllTasks)
			r.Put("/bulk-expire", bulkExpireTasks)
			r.Put("/bulk-archive", bulkArchiveTasks)
			r.Get("/{taskID}", getTask)
			r.Put("/{taskID}", updateTask)
			r.Delete("/{taskID}", cancelTask)
			r.Put("/{taskID}/archive", archiveTask)
			r.Put("/{taskID}/unarchive", unarchiveTask)
		})

		// Agents
		r.Route("/agents", func(r chi.Router) {
			r.Get("/", listAgents)
			r.Get("/{agentID}/status", getAgentStatus)
			r.Post("/{agentID}/status", reportAgentStatus) // Self-report status
			r.Post("/{agentID}/reset", resetAgent)
			r.Post("/{agentID}/message", sendAgentMessage)
			r.Post("/init", initializeAgents) // Send initial instructions to all agents
			r.Post("/start", startAgents)
			r.Post("/stop", stopAgents)
		})

		// Teams (multi-session manager management, backward compat endpoint names)
		r.Route("/teams", func(r chi.Router) {
			r.Get("/", listManagers)
			r.Post("/{teamLead}/message", sendManagerMessageHandler)
			r.Post("/{teamLead}/start", startManagerHandler)
			r.Post("/{teamLead}/stop", stopManagerHandler)
			r.Get("/{teamLead}/status", getManagerStatusHandler)
		})

		// Discussions (agent collaboration)
		r.Route("/discussions", func(r chi.Router) {
			r.Get("/", listDiscussions)
			r.Post("/", createDiscussion)
			r.Get("/{discussionID}", getDiscussion)
			r.Post("/{discussionID}/reply", replyToDiscussion)
			r.Put("/{discussionID}/close", closeDiscussion)
		})

		// Queue (communication log)
		r.Route("/queue", func(r chi.Router) {
			r.Get("/", getQueueMessages)
			r.Post("/simulate", simulateTask)
		})

		// System status (Claude agents, watcher, etc.)
		r.Get("/system/status", getSystemStatus)

		// System upgrade (selective layer upgrade)
		upgradeBaseDir, _ := filepath.Abs(".")
		upgradeHandler := handler.NewUpgradeHandler(upgradeBaseDir)
		r.Post("/upgrade", upgradeHandler.HandleUpgrade)
		r.Post("/system/upgrade", upgradeHandler.HandleUpgrade)

		// Feature Requests
		r.Route("/requests", func(r chi.Router) {
			r.Get("/", listRequests)
			r.Post("/", createRequest)
			r.Get("/{requestID}", getRequest)
			r.Put("/{requestID}", updateRequest)
			r.Delete("/{requestID}", deleteRequest)
			r.Post("/{requestID}/execute", executeRequest)
		})

		// Webhooks (incoming)
		r.Route("/webhooks", func(r chi.Router) {
			r.Post("/github", handleGitHubWebhook)
		})

		// GitHub API (proxy for frontend)
		r.Route("/github", func(r chi.Router) {
			r.Get("/summary", getGitHubSummary)
			r.Post("/refresh", refreshGitHubSummary)
		})

		// Pending Questions (Claude asking for user input)
		r.Route("/questions", func(r chi.Router) {
			r.Get("/", listPendingQuestions)
			r.Post("/", createPendingQuestion)
			r.Get("/{questionID}", getPendingQuestion)
			r.Post("/{questionID}/answer", answerPendingQuestion)
		})

		// Usage (ccusage CLI wrapper)
		r.Get("/usage", handleGetUsage)

		// Strategies Management (trading strategies via 1Password)
		strategyHandler := handler.NewStrategyHandler(configDir)
		r.Route("/strategies", func(r chi.Router) {
			r.Get("/", strategyHandler.HandleList)
			r.Get("/{strategyID}", strategyHandler.HandleGet)
			r.Post("/{strategyID}/toggle", strategyHandler.HandleToggle)
			r.Put("/{strategyID}/status", strategyHandler.HandleSetStatus)
			r.Get("/{strategyID}/params", strategyHandler.HandleGetParams)
			r.Put("/{strategyID}/params", strategyHandler.HandleUpdateParams)
		})

		// Sessions Management (tmux sessions)
		sessionHandler := handler.NewSessionHandler()
		r.Route("/sessions", func(r chi.Router) {
			r.Get("/", sessionHandler.HandleList)
			r.Post("/", sessionHandler.HandleCreate)
			r.Delete("/{sessionName}", sessionHandler.HandleDelete)
			r.Post("/{sessionName}/restart", sessionHandler.HandleRestart)
			r.Patch("/{sessionName}/description", sessionHandler.HandleUpdateDescription)
			r.Get("/agents", sessionHandler.HandleListAgents)
			r.Get("/cli-types", sessionHandler.HandleListCliTypes)
			r.Get("/pool-status", sessionHandler.HandlePoolStatus)
		})

		// Triggers (cron job manual execution)
		repoRoot, _ := filepath.Abs(".")
		triggerHandler := handler.NewTriggerHandler(repoRoot)
		r.Route("/triggers", func(r chi.Router) {
			r.Get("/status", triggerHandler.HandleStatus)
			r.Post("/cron/{jobName}", triggerHandler.HandleTrigger)
			r.Post("/update-resources", triggerHandler.HandleUpdateResources)
		})

		// Cron Jobs Management
		cronHandler := handler.NewCronHandler(cronJobRepo)
		r.Route("/crons", func(r chi.Router) {
			r.Get("/", cronHandler.HandleList)
			r.Post("/", cronHandler.HandleCreate)
			r.Get("/{jobID}", cronHandler.HandleGet)
			r.Put("/{jobID}", cronHandler.HandleUpdate)
			r.Delete("/{jobID}", cronHandler.HandleDelete)
		})

		// LLM Providers
		providerHandler := handler.NewProviderHandler()
		r.Route("/providers", func(r chi.Router) {
			r.Get("/", providerHandler.HandleListProviders)
			r.Post("/", providerHandler.HandleCreateProvider)
			r.Put("/{providerID}", providerHandler.HandleUpdateProvider)
			r.Get("/agent-configs", providerHandler.HandleListAgentConfigs)
			r.Put("/agents/{agentID}/config", providerHandler.HandleUpdateAgentConfig)
		})

		// Revenue / KPI / Activity / Targets
		revenueHandler := handler.NewRevenueHandler(revenueRepo)
		r.Route("/revenue", func(r chi.Router) {
			r.Get("/", revenueHandler.HandleGetRevenue)
			r.Post("/", revenueHandler.HandleCreateRevenue)
		})
		r.Route("/kpi", func(r chi.Router) {
			r.Get("/latest", revenueHandler.HandleGetKpi)
			r.Post("/", revenueHandler.HandleCreateKpi)
		})
		r.Route("/activity", func(r chi.Router) {
			r.Get("/", revenueHandler.HandleGetActivity)
			r.Post("/", revenueHandler.HandleCreateActivity)
		})
		r.Route("/targets", func(r chi.Router) {
			r.Get("/", revenueHandler.HandleGetTargets)
			r.Post("/", revenueHandler.HandleCreateTarget)
		})

		// App Management (multi-repo)
		r.Route("/apps", func(r chi.Router) {
			r.Get("/", listAppsHandler)
			r.Post("/refresh", refreshAllAppsHandler)
			r.Get("/{appID}", getAppHandler)
			r.Post("/{appID}/build", buildAppHandler)
			r.Post("/{appID}/start", startAppHandler)
			r.Post("/{appID}/stop", stopAppHandler)
			r.Post("/{appID}/restart", restartAppHandler)
			r.Get("/{appID}/status", getAppStatusHandler)
			r.Get("/{appID}/logs", getAppLogsHandler)
			r.Get("/{appID}/dashboard/*", dashboardProxyHandler)
			r.Get("/{appID}/dashboard", dashboardProxyHandler)
			r.Get("/{appID}/data/trades", getAppTradesHandler)
		})

		// Configuration
		r.Route("/config", func(r chi.Router) {
			r.Get("/agents", getAgentsConfig)
			r.Get("/limits", getLimitsConfig)
		})

		// RAG (Retrieval-Augmented Generation)
		ragBaseDir, _ := filepath.Abs(".")
		ragClient := service.NewRAGClient(ragBaseDir)
		ragHandler := handler.NewRAGHandler(ragClient)
		r.Route("/rag", func(r chi.Router) {
			r.Get("/query", ragHandler.HandleQuery)
		})

		// Codex Executor
		codexExecutor := service.NewCodexExecutor(2)
		codexHandler := handler.NewCodexHandler(codexExecutor)
		r.Route("/codex", func(r chi.Router) {
			r.Post("/tasks", codexHandler.HandleSubmit)
			r.Get("/tasks", codexHandler.HandleListTasks)
			r.Get("/tasks/{id}", codexHandler.HandleGetTask)
			r.Get("/stats", codexHandler.HandleStats)
		})

		messageHandler := handler.NewMessageHandler(sendTmuxMessageToSession, hasTmuxSession, writeMessageToFile)
		r.Route("/messages", func(r chi.Router) {
			r.Post("/send", messageHandler.HandleSend)
		})

	})

	// A2A Protocol Agent Card
	agentCardHandler := handler.NewAgentCardHandler()
	r.Get("/.well-known/agent.json", agentCardHandler.HandleGetAgentCard)

	// Serve static frontend files (for production mode without Docker)
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "./frontend/dist"
	}
	if _, err := os.Stat(staticDir); err == nil {
		log.Printf("Serving static files from %s", staticDir)
		fileServer := http.FileServer(http.Dir(staticDir))
		r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
			// Disable caching for HTML files (SPA)
			if req.URL.Path == "/" || req.URL.Path == "/index.html" {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			}
			// Try to serve the file
			path := staticDir + req.URL.Path
			if _, err := os.Stat(path); os.IsNotExist(err) {
				// If file doesn't exist, serve index.html (SPA fallback)
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				http.ServeFile(w, req, staticDir+"/index.html")
				return
			}
			fileServer.ServeHTTP(w, req)
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	bind := os.Getenv("BIND_ADDR")
	if bind == "" {
		bind = "127.0.0.1" // ローカルのみ (MacBook版)
	}

	log.Printf("Server starting on %s:%s", bind, port)
	srv := &http.Server{
		Addr:         bind + ":" + port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 35 * time.Minute,
		IdleTimeout:  5 * time.Minute,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// checkTmuxPaneStatus checks if Claude is running in a tmux pane
// Uses the same proven approach as getSystemStatus (tmux list-panes)
func checkTmuxPaneStatus(pane int) (bool, string) {
	// Use list-panes with index format to get pane index and command
	cmd := exec.Command("tmux", "list-panes", "-t", tmuxDefaultSessionName, "-F", "#{pane_index}:#{pane_current_command}")
	output, err := cmd.Output()
	if err != nil {
		return false, ""
	}

	// Parse output to find the specific pane
	lines := splitLines(string(output))
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Format is "index:command"
		colonIdx := -1
		for i := 0; i < len(line); i++ {
			if line[i] == ':' {
				colonIdx = i
				break
			}
		}
		if colonIdx == -1 {
			continue
		}

		// Parse pane index
		paneIdxStr := line[:colonIdx]
		paneIdx := 0
		for _, c := range paneIdxStr {
			if c >= '0' && c <= '9' {
				paneIdx = paneIdx*10 + int(c-'0')
			}
		}

		if paneIdx == pane {
			command := line[colonIdx+1:]
			// Trim whitespace
			for len(command) > 0 && (command[len(command)-1] == '\n' || command[len(command)-1] == ' ') {
				command = command[:len(command)-1]
			}
			isRunning := command == "claude"
			return isRunning, command
		}
	}
	return false, ""
}

// getAgentInfoList returns a list of all agents with their status (for Discord bot)
func getAgentInfoList() []AgentInfo {
	var agents []AgentInfo

	// Helper to get agent status using tmux pane check
	getStatusWithTmux := func(agentID string, pane int) (string, string) {
		isClaudeRunning, _ := checkTmuxPaneStatus(pane)

		agentStatusLock.RLock()
		memStatus, exists := agentStatuses[agentID]
		agentStatusLock.RUnlock()

		var status, currentTask string
		if !isClaudeRunning {
			status = "stopped"
		} else if exists {
			status = memStatus.Status
			currentTask = memStatus.CurrentTask
			if time.Since(memStatus.LastHeartbeat) > statusTimeout {
				status = "running"
			}
		} else {
			status = "running"
		}
		return status, currentTask
	}

	// Helper to get agent status from self-report only (for teammates without tmux pane)
	getStatusFromSelfReport := func(agentID string) (string, string) {
		agentStatusLock.RLock()
		memStatus, exists := agentStatuses[agentID]
		agentStatusLock.RUnlock()

		if exists {
			if time.Since(memStatus.LastHeartbeat) > statusTimeout {
				return "stopped", ""
			}
			return memStatus.Status, memStatus.CurrentTask
		}
		return "unknown", ""
	}

	isAgentTeamsMode := appConfig != nil && appConfig.Agents != nil &&
		appConfig.Agents.Mode == "agent_teams" &&
		appConfig.Agents.AgentTeams != nil && appConfig.Agents.AgentTeams.Enabled

	if isAgentTeamsMode {
		// Agent Teams mode: team_lead + dynamic teammates

		// Team Lead: uses manager's pane 0
		if appConfig.Agents.Manager.Enabled {
			managerPane := appConfig.Agents.Manager.Pane
			status, currentTask := getStatusWithTmux("manager", managerPane)
			agents = append(agents, AgentInfo{
				ID:          "team-lead",
				Role:        "team_lead",
				Status:      status,
				CurrentTask: currentTask,
			})
		}

		// Teammates: use workers config as teammate definitions
		type workerEntry struct {
			id   string
			pane int
		}
		var workerEntries []workerEntry
		for id, w := range appConfig.Agents.Workers {
			if w.Enabled {
				workerEntries = append(workerEntries, workerEntry{id: id, pane: w.Pane})
			}
		}
		sort.Slice(workerEntries, func(i, j int) bool {
			return workerEntries[i].pane < workerEntries[j].pane
		})

		for _, entry := range workerEntries {
			status, currentTask := getStatusFromSelfReport(entry.id)
			if status == "unknown" {
				status = "waiting"
			}
			agents = append(agents, AgentInfo{
				ID:          entry.id,
				Role:        "teammate",
				Status:      status,
				CurrentTask: currentTask,
			})
		}
	} else {
		// Legacy mode: manager + workers

		// Add manager if enabled
		if appConfig != nil && appConfig.Agents != nil && appConfig.Agents.Manager.Enabled {
			managerPane := appConfig.Agents.Manager.Pane
			status, currentTask := getStatusWithTmux("manager", managerPane)
			agents = append(agents, AgentInfo{
				ID:          "manager",
				Role:        "manager",
				Status:      status,
				CurrentTask: currentTask,
			})
		}

		// Add workers from config
		if appConfig != nil {
			for _, workerID := range appConfig.GetEnabledWorkers() {
				workerConfig := appConfig.Agents.Workers[workerID]
				status, currentTask := getStatusWithTmux(workerID, workerConfig.Pane)
				agents = append(agents, AgentInfo{
					ID:          workerID,
					Role:        "worker",
					Status:      status,
					CurrentTask: currentTask,
				})
			}
		}
	}

	return agents
}

func listAgents(w http.ResponseWriter, r *http.Request) {
	agents := []map[string]interface{}{}

	// Helper function to get agent status combining self-report and tmux check
	getStatusWithTmux := func(agentID string, pane int) map[string]interface{} {
		// First, check if Claude is actually running in the tmux pane
		isClaudeRunning, _ := checkTmuxPaneStatus(pane)

		agentStatusLock.RLock()
		memStatus, exists := agentStatuses[agentID]
		agentStatusLock.RUnlock()

		// Determine status based on tmux check and self-report
		var status string
		var currentTask, taskDesc interface{}
		var lastHeartbeat string

		if !isClaudeRunning {
			status = "stopped"
			currentTask = nil
			taskDesc = nil
		} else if exists {
			status = memStatus.Status
			currentTask = memStatus.CurrentTask
			taskDesc = memStatus.TaskDescription
			lastHeartbeat = memStatus.LastHeartbeat.Format(time.RFC3339)
			if time.Since(memStatus.LastHeartbeat) > statusTimeout {
				status = "running"
			}
		} else {
			status = "running"
			currentTask = nil
			taskDesc = nil
		}

		return map[string]interface{}{
			"status":                   status,
			"current_task":             currentTask,
			"current_task_description": taskDesc,
			"last_heartbeat":           lastHeartbeat,
		}
	}

	// Helper function to get agent status from self-report only (for teammates without tmux pane)
	getStatusFromSelfReport := func(agentID string) map[string]interface{} {
		agentStatusLock.RLock()
		memStatus, exists := agentStatuses[agentID]
		agentStatusLock.RUnlock()

		if exists {
			status := memStatus.Status
			if time.Since(memStatus.LastHeartbeat) > statusTimeout {
				status = "stopped"
			}
			return map[string]interface{}{
				"status":                   status,
				"current_task":             memStatus.CurrentTask,
				"current_task_description": memStatus.TaskDescription,
				"last_heartbeat":           memStatus.LastHeartbeat.Format(time.RFC3339),
			}
		}
		return map[string]interface{}{
			"status":                   "unknown",
			"current_task":             nil,
			"current_task_description": nil,
			"last_heartbeat":           "",
		}
	}

	isAgentTeamsMode := appConfig.Agents != nil &&
		appConfig.Agents.Mode == "agent_teams" &&
		appConfig.Agents.AgentTeams != nil && appConfig.Agents.AgentTeams.Enabled

	var teamName string
	if isAgentTeamsMode {
		teamName = appConfig.Agents.AgentTeams.TeamName
	}

	if isAgentTeamsMode {
		// Agent Teams mode: team_lead + dynamic teammates

		// Team Lead: uses manager's pane 0 for tmux status check
		if appConfig.Agents != nil && appConfig.Agents.Manager.Enabled {
			managerPane := appConfig.Agents.Manager.Pane
			leadStatus := getStatusWithTmux("manager", managerPane)
			agents = append(agents, map[string]interface{}{
				"id":                       "team-lead",
				"role":                     "team_lead",
				"description":              "Team Lead - タスク分配・進捗管理・全体調整",
				"status":                   leadStatus["status"],
				"current_task":             leadStatus["current_task"],
				"current_task_description": leadStatus["current_task_description"],
				"started_at":               leadStatus["last_heartbeat"],
				"agent_type":               "team_lead",
				"team_name":                teamName,
			})
		}

		// Teammates: use workers config as teammate definitions
		type workerEntry struct {
			id          string
			pane        int
			description string
		}
		var workerEntries []workerEntry
		for id, w := range appConfig.Agents.Workers {
			if w.Enabled {
				workerEntries = append(workerEntries, workerEntry{id: id, pane: w.Pane, description: w.Description})
			}
		}
		sort.Slice(workerEntries, func(i, j int) bool {
			return workerEntries[i].pane < workerEntries[j].pane
		})

		for _, entry := range workerEntries {
			tmStatus := getStatusFromSelfReport(entry.id)
			status := tmStatus["status"]
			if status == "unknown" {
				status = "waiting"
			}
			agents = append(agents, map[string]interface{}{
				"id":                       entry.id,
				"role":                     "teammate",
				"description":              entry.description,
				"status":                   status,
				"current_task":             tmStatus["current_task"],
				"current_task_description": tmStatus["current_task_description"],
				"started_at":               tmStatus["last_heartbeat"],
				"agent_type":               "teammate",
				"team_name":                teamName,
			})
		}
	} else {
		// Legacy mode: manager + workers

		// Add manager if enabled in config
		if appConfig.Agents != nil && appConfig.Agents.Manager.Enabled {
			managerPane := appConfig.Agents.Manager.Pane
			managerStatus := getStatusWithTmux("manager", managerPane)
			agents = append(agents, map[string]interface{}{
				"id":                       "manager",
				"role":                     "manager",
				"status":                   managerStatus["status"],
				"current_task":             managerStatus["current_task"],
				"current_task_description": managerStatus["current_task_description"],
				"started_at":               managerStatus["last_heartbeat"],
				"agent_type":               "independent",
				"team_name":                "",
			})
		}

		// Add workers from config
		for _, workerID := range appConfig.GetEnabledWorkers() {
			workerConfig := appConfig.Agents.Workers[workerID]
			workerStatus := getStatusWithTmux(workerID, workerConfig.Pane)
			agents = append(agents, map[string]interface{}{
				"id":                       workerID,
				"role":                     "worker",
				"description":              workerConfig.Description,
				"status":                   workerStatus["status"],
				"current_task":             workerStatus["current_task"],
				"current_task_description": workerStatus["current_task_description"],
				"started_at":               workerStatus["last_heartbeat"],
				"agent_type":               "independent",
				"team_name":                "",
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{"agents": agents}
	if isAgentTeamsMode {
		maxTeammates := 4
		if appConfig.Agents.AgentTeams.MaxTeammates > 0 {
			maxTeammates = appConfig.Agents.AgentTeams.MaxTeammates
		}
		response["team_info"] = map[string]interface{}{
			"mode":          "agent_teams",
			"max_teammates": maxTeammates,
		}
	}
	json.NewEncoder(w).Encode(response)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func getAgentStatus(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")

	// Get pane index for this agent
	pane := appConfig.GetWorkerPaneIndex(agentID)

	// Check if Claude is running in tmux
	isClaudeRunning := false
	if pane >= 0 {
		isClaudeRunning, _ = checkTmuxPaneStatus(pane)
	}

	// Check memory-based status
	agentStatusLock.RLock()
	memStatus, exists := agentStatuses[agentID]
	agentStatusLock.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	// Determine status based on tmux and self-report
	if !isClaudeRunning {
		// Claude is not running
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":                   "stopped",
			"current_task":             nil,
			"current_task_description": nil,
		})
		return
	}

	if exists {
		// Claude is running, use self-reported status
		currentStatus := memStatus.Status
		if time.Since(memStatus.LastHeartbeat) > statusTimeout {
			currentStatus = "running" // Claude alive but not self-reporting
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":                   currentStatus,
			"current_task":             memStatus.CurrentTask,
			"current_task_description": memStatus.TaskDescription,
			"last_heartbeat":           memStatus.LastHeartbeat.Format(time.RFC3339),
		})
		return
	}

	// Claude is running but no status reported yet
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":                   "running",
		"current_task":             nil,
		"current_task_description": nil,
	})
}

// ReportStatusRequest represents a status report from an agent
type ReportStatusRequest struct {
	Status          string `json:"status"`           // available, busy
	CurrentTask     string `json:"current_task"`     // task ID if busy
	TaskDescription string `json:"task_description"` // task description if busy
}

// reportAgentStatus handles agent self-reporting their status
func reportAgentStatus(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")

	// Validate agent ID using config
	if !appConfig.IsValidAgent(agentID) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("unknown agent: %s", agentID),
		})
		return
	}

	// Parse request body
	var req ReportStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "invalid request body",
		})
		return
	}

	// Validate status
	if req.Status != "available" && req.Status != "busy" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "status must be 'available' or 'busy'",
		})
		return
	}

	// Update in-memory status
	agentStatusLock.Lock()
	agentStatuses[agentID] = &AgentStatus{
		Status:          req.Status,
		CurrentTask:     req.CurrentTask,
		TaskDescription: req.TaskDescription,
		LastHeartbeat:   time.Now(),
	}
	agentStatusLock.Unlock()

	// Also persist to database if available
	if agentRepo != nil {
		var currentTask, taskDesc *string
		if req.CurrentTask != "" {
			currentTask = &req.CurrentTask
		}
		if req.TaskDescription != "" {
			taskDesc = &req.TaskDescription
		}
		if err := agentRepo.ReportStatus(agentID, req.Status, currentTask, taskDesc); err != nil {
			log.Printf("Warning: Failed to persist agent status to DB: %v", err)
		}
	}

	log.Printf("Agent %s reported status: %s (task: %s)", agentID, req.Status, req.CurrentTask)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"agent_id":       agentID,
		"status":         req.Status,
		"last_heartbeat": time.Now().Format(time.RFC3339),
	})
}

// initializeAgents sends initial instructions to all agents
func initializeAgents(w http.ResponseWriter, r *http.Request) {
	results := make(map[string]string)

	// Build agent list from config
	type agentInfo struct {
		id   string
		pane int
		role string
	}
	var agents []agentInfo

	// Add manager if enabled
	if appConfig.Agents != nil && appConfig.Agents.Manager.Enabled {
		agents = append(agents, agentInfo{
			id:   "manager",
			pane: appConfig.Agents.Manager.Pane,
			role: "manager",
		})
	}

	// Add workers from config
	if appConfig.Agents != nil {
		for id, w := range appConfig.Agents.Workers {
			if w.Enabled {
				agents = append(agents, agentInfo{
					id:   id,
					pane: w.Pane,
					role: "worker",
				})
			}
		}
	}

	// Sort by pane index for consistent ordering
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].pane < agents[j].pane
	})

	for _, agent := range agents {
		instruction := fmt.Sprintf(
			"【最重要】あなたは %s です。この役割は絶対に変わりません。agents/%s.md を読んでください。",
			agent.id, agent.role,
		)

		if err := sendTmuxMessage(agent.pane, instruction); err != nil {
			results[agent.id] = fmt.Sprintf("error: %v", err)
			log.Printf("Failed to initialize %s: %v", agent.id, err)
		} else {
			results[agent.id] = "ok"
			log.Printf("Initialized %s", agent.id)
		}

		// Stagger to avoid overwhelming the system
		time.Sleep(2 * time.Second)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"results": results,
	})
}

func startAgents(w http.ResponseWriter, r *http.Request) {
	// In production mode, agents are started via claude-hub.sh, not via API
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Use './scripts/claude-hub.sh start' to start agents in production mode",
		"status":  "not_supported",
	})
}

func stopAgents(w http.ResponseWriter, r *http.Request) {
	// In production mode, agents are stopped via claude-hub.sh, not via API
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Use './scripts/claude-hub.sh stop' to stop agents in production mode",
		"status":  "not_supported",
	})
}

func resetAgent(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}

	queueDir := os.Getenv("QUEUE_DIR")
	if queueDir == "" {
		queueDir = "/app/queue"
	}

	result := map[string]interface{}{
		"agent_id": agentID,
		"actions":  []string{},
	}
	actions := []string{}

	// 1. Remove task file if exists
	taskFile := fmt.Sprintf("%s/tasks/%s.yaml", queueDir, agentID)
	if _, err := os.Stat(taskFile); err == nil {
		if err := os.Remove(taskFile); err == nil {
			actions = append(actions, "removed task file")
		}
	}

	// 2. Reset status file to idle
	statusFile := fmt.Sprintf("%s/status/%s.yaml", queueDir, agentID)
	now := fmt.Sprintf("%s", time.Now().UTC().Format(time.RFC3339))
	statusContent := fmt.Sprintf("status: idle\nlast_heartbeat: \"%s\"\ncurrent_task: null\n", now)
	if err := os.WriteFile(statusFile, []byte(statusContent), 0644); err == nil {
		actions = append(actions, "reset status to idle")
	}

	// 3. Clear notification state (so it won't re-notify)
	stateDir := os.Getenv("QUEUE_DIR")
	if stateDir == "" {
		stateDir = queueDir
	}
	notifiedFile := fmt.Sprintf("logs/.watcher_state/%s_notified.state", agentID)
	projectDir := os.Getenv("PROJECT_DIR")
	if projectDir != "" {
		notifiedFile = fmt.Sprintf("%s/logs/.watcher_state/%s_notified.state", projectDir, agentID)
	}
	os.Remove(notifiedFile)
	actions = append(actions, "cleared notification state")

	result["actions"] = actions
	result["status"] = "reset"

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// Task handlers
func listTasks(w http.ResponseWriter, r *http.Request) {
	if taskRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	status := r.URL.Query().Get("status")
	hideCompleted := r.URL.Query().Get("hide_completed") == "true"
	showArchived := r.URL.Query().Get("show_archived") == "true"

	// Pagination parameters
	page := 1
	perPage := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 100 {
			perPage = parsed
		}
	}
	offset := (page - 1) * perPage

	// Get total count (with filter)
	total, err := taskRepo.CountFiltered(status, hideCompleted, showArchived)
	if err != nil {
		log.Printf("Failed to count tasks: %v", err)
		total = 0
	}

	dbTasks, err := taskRepo.ListWithPaginationFiltered(status, perPage, offset, hideCompleted, showArchived)
	if err != nil {
		log.Printf("Failed to list tasks from DB: %v", err)
		http.Error(w, "Failed to list tasks", http.StatusInternalServerError)
		return
	}

	// Convert database tasks to API format
	tasks := make([]map[string]interface{}, len(dbTasks))
	for i, t := range dbTasks {
		tasks[i] = map[string]interface{}{
			"task_id":     t.ID,
			"type":        t.Type,
			"priority":    t.Priority,
			"description": t.Description,
			"status":      t.Status,
			"assigned_to": t.AssignedTo,
			"source":      t.Source,
			"archived":    t.Archived,
			"created_at":  t.CreatedAt.Format(time.RFC3339),
		}
		if t.AssignedAt != nil {
			tasks[i]["assigned_at"] = t.AssignedAt.Format(time.RFC3339)
		}
		if t.CompletedAt != nil {
			tasks[i]["completed_at"] = t.CompletedAt.Format(time.RFC3339)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": tasks,
		"pagination": map[string]interface{}{
			"page":        page,
			"per_page":    perPage,
			"total":       total,
			"total_pages": (total + perPage - 1) / perPage,
		},
	})
}

func createTask(w http.ResponseWriter, r *http.Request) {
	if taskRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Type        string `json:"type"`
		Priority    string `json:"priority"`
		Description string `json:"description"`
		Repository  string `json:"repository,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Description == "" {
		http.Error(w, "description is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Type == "" {
		req.Type = "development"
	}
	if req.Priority == "" {
		req.Priority = "medium"
	}

	// リポジトリが指定された場合、descriptionにプレフィックスとして付与
	description := req.Description
	if req.Repository != "" {
		description = fmt.Sprintf("[repo:%s] %s", req.Repository, req.Description)
	}

	// Create task in database
	taskID := fmt.Sprintf("TASK-%d", time.Now().UnixNano())
	dbTask := &database.Task{
		ID:          taskID,
		Type:        req.Type,
		Priority:    req.Priority,
		Description: description,
		Status:      "pending",
		Source:      "api",
		CreatedAt:   time.Now(),
	}

	if err := taskRepo.Create(dbTask); err != nil {
		log.Printf("Failed to create task in DB: %v", err)
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// Notify manager via tmux about the new task
	go func() {
		notification := fmt.Sprintf("新規タスク: %s - %s (優先度: %s). APIで GET /api/v1/tasks/%s を確認してください。",
			taskID, req.Description, req.Priority, taskID)
		if err := sendTmuxMessage(0, notification); err != nil {
			log.Printf("Failed to notify manager about new task: %v", err)
		} else {
			log.Printf("Notified manager about new task: %s", taskID)
		}
	}()

	// Return created task
	response := map[string]interface{}{
		"task_id":     dbTask.ID,
		"type":        dbTask.Type,
		"priority":    dbTask.Priority,
		"description": dbTask.Description,
		"status":      dbTask.Status,
		"assigned_to": dbTask.AssignedTo,
		"source":      dbTask.Source,
		"created_at":  dbTask.CreatedAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func getTask(w http.ResponseWriter, r *http.Request) {
	if taskRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	taskID := chi.URLParam(r, "taskID")
	dbTask, err := taskRepo.GetByID(taskID)
	if err != nil {
		log.Printf("Failed to get task from DB: %v", err)
		http.Error(w, "Failed to get task", http.StatusInternalServerError)
		return
	}
	if dbTask == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	result := map[string]interface{}{
		"task_id":     dbTask.ID,
		"type":        dbTask.Type,
		"priority":    dbTask.Priority,
		"description": dbTask.Description,
		"status":      dbTask.Status,
		"assigned_to": dbTask.AssignedTo,
		"source":      dbTask.Source,
		"archived":    dbTask.Archived,
		"created_at":  dbTask.CreatedAt.Format(time.RFC3339),
	}
	if dbTask.AssignedAt != nil {
		result["assigned_at"] = dbTask.AssignedAt.Format(time.RFC3339)
	}
	if dbTask.CompletedAt != nil {
		result["completed_at"] = dbTask.CompletedAt.Format(time.RFC3339)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// UpdateTaskRequest represents a request to update a task
type UpdateTaskRequest struct {
	Status     string  `json:"status"`      // assigned, in_progress, completed, cancelled
	AssignedTo *string `json:"assigned_to"` // worker ID if assigning
}

func updateTask(w http.ResponseWriter, r *http.Request) {
	if taskRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	taskID := chi.URLParam(r, "taskID")

	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"pending": true, "assigned": true, "in_progress": true,
		"completed": true, "cancelled": true, "archived": true,
	}
	if req.Status != "" && !validStatuses[req.Status] {
		http.Error(w, "Invalid status", http.StatusBadRequest)
		return
	}

	// Get current task
	task, err := taskRepo.GetByID(taskID)
	if err != nil {
		log.Printf("Failed to get task from DB: %v", err)
		http.Error(w, "Failed to get task", http.StatusInternalServerError)
		return
	}
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Update status if provided
	if req.Status != "" {
		if err := taskRepo.UpdateStatus(taskID, req.Status); err != nil {
			log.Printf("Failed to update task status: %v", err)
			http.Error(w, "Failed to update task", http.StatusInternalServerError)
			return
		}
		task.Status = req.Status

	}

	// Update assignment if provided
	if req.AssignedTo != nil {
		if err := taskRepo.Assign(taskID, *req.AssignedTo); err != nil {
			log.Printf("Failed to assign task: %v", err)
			http.Error(w, "Failed to assign task", http.StatusInternalServerError)
			return
		}
		task.AssignedTo = req.AssignedTo
		task.Status = "assigned"
	}

	log.Printf("Task %s updated: status=%s, assigned_to=%v", taskID, task.Status, task.AssignedTo)

	result := map[string]interface{}{
		"task_id":     task.ID,
		"type":        task.Type,
		"priority":    task.Priority,
		"description": task.Description,
		"status":      task.Status,
		"assigned_to": task.AssignedTo,
		"source":      task.Source,
		"archived":    task.Archived,
		"created_at":  task.CreatedAt.Format(time.RFC3339),
	}
	if task.AssignedAt != nil {
		result["assigned_at"] = task.AssignedAt.Format(time.RFC3339)
	}
	if task.CompletedAt != nil {
		result["completed_at"] = task.CompletedAt.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func cancelTask(w http.ResponseWriter, r *http.Request) {
	if taskRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	taskID := chi.URLParam(r, "taskID")
	purge := r.URL.Query().Get("purge") == "true"

	// Check if task exists
	task, err := taskRepo.GetByID(taskID)
	if err != nil {
		log.Printf("Failed to get task from DB: %v", err)
		http.Error(w, "Failed to cancel task", http.StatusInternalServerError)
		return
	}
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Purge mode: physically delete the task
	if purge {
		if err := taskRepo.Delete(taskID); err != nil {
			log.Printf("Failed to delete task from DB: %v", err)
			http.Error(w, "Failed to delete task", http.StatusInternalServerError)
			return
		}
		log.Printf("Task %s purged (physically deleted)", taskID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "purged", "task_id": taskID})
		return
	}

	// Normal mode: cancel the task (status change only)
	if task.Status == "completed" || task.Status == "cancelled" || task.Status == "archived" {
		http.Error(w, fmt.Sprintf("Task already %s", task.Status), http.StatusBadRequest)
		return
	}

	// Update status in database
	if err := taskRepo.UpdateStatus(taskID, "cancelled"); err != nil {
		log.Printf("Failed to update task status in DB: %v", err)
		http.Error(w, "Failed to cancel task", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled", "task_id": taskID})
}

func cancelAllTasks(w http.ResponseWriter, r *http.Request) {
	if taskRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	purge := r.URL.Query().Get("purge") == "true"
	statusParam := r.URL.Query().Get("status")

	// Purge mode: physically delete tasks by status
	if purge {
		var statuses []string
		if statusParam != "" {
			// Parse comma-separated status values
			for _, s := range splitByComma(statusParam) {
				s = trimSpace(s)
				if s != "" {
					statuses = append(statuses, s)
				}
			}
		} else {
			// Default: purge completed, cancelled, and archived tasks
			statuses = []string{"completed", "cancelled", "archived"}
		}

		if len(statuses) == 0 {
			http.Error(w, "No valid status specified", http.StatusBadRequest)
			return
		}

		count, err := taskRepo.DeleteByStatus(statuses...)
		if err != nil {
			log.Printf("Failed to purge tasks from DB: %v", err)
			http.Error(w, "Failed to purge tasks", http.StatusInternalServerError)
			return
		}

		log.Printf("Purged %d tasks from DB (statuses: %v)", count, statuses)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "purged",
			"count":    count,
			"statuses": statuses,
		})
		return
	}

	// Normal mode: delete pending/assigned/in_progress tasks
	count, err := taskRepo.DeleteByStatus("pending", "assigned", "in_progress")
	if err != nil {
		log.Printf("Failed to delete tasks from DB: %v", err)
		http.Error(w, "Failed to cancel tasks", http.StatusInternalServerError)
		return
	}

	log.Printf("Cancelled %d tasks from DB", count)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "cancelled", "count": count})
}

func archiveTask(w http.ResponseWriter, r *http.Request) {
	if taskRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	taskID := chi.URLParam(r, "taskID")
	task, err := taskRepo.GetByID(taskID)
	if err != nil {
		log.Printf("Failed to get task from DB: %v", err)
		http.Error(w, "Failed to archive task", http.StatusInternalServerError)
		return
	}
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if err := taskRepo.ArchiveTask(taskID); err != nil {
		log.Printf("Failed to archive task: %v", err)
		http.Error(w, "Failed to archive task", http.StatusInternalServerError)
		return
	}

	log.Printf("Task %s archived", taskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "archived", "task_id": taskID})
}

func unarchiveTask(w http.ResponseWriter, r *http.Request) {
	if taskRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	taskID := chi.URLParam(r, "taskID")
	task, err := taskRepo.GetByID(taskID)
	if err != nil {
		log.Printf("Failed to get task from DB: %v", err)
		http.Error(w, "Failed to unarchive task", http.StatusInternalServerError)
		return
	}
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if err := taskRepo.UnarchiveTask(taskID); err != nil {
		log.Printf("Failed to unarchive task: %v", err)
		http.Error(w, "Failed to unarchive task", http.StatusInternalServerError)
		return
	}

	log.Printf("Task %s unarchived", taskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "unarchived", "task_id": taskID})
}

func bulkExpireTasks(w http.ResponseWriter, r *http.Request) {
	if taskRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		OlderThanHours int `json:"older_than_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.OlderThanHours <= 0 {
		req.OlderThanHours = 24
	}

	count, err := taskRepo.CloseOldPendingTasks(req.OlderThanHours)
	if err != nil {
		log.Printf("Failed to expire old tasks: %v", err)
		http.Error(w, "Failed to expire tasks", http.StatusInternalServerError)
		return
	}

	log.Printf("Expired %d pending tasks older than %d hours", count, req.OlderThanHours)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "expired",
		"count":            count,
		"older_than_hours": req.OlderThanHours,
	})
}

func bulkArchiveTasks(w http.ResponseWriter, r *http.Request) {
	if taskRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		OlderThanHours *int   `json:"older_than_hours"`
		Status         string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var count int64
	var err error

	if req.OlderThanHours != nil {
		count, err = taskRepo.ArchiveOldTasks(*req.OlderThanHours)
	} else if req.Status != "" {
		count, err = taskRepo.ArchiveByStatus(req.Status)
	} else {
		http.Error(w, "Either older_than_hours or status is required", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("Failed to bulk archive tasks: %v", err)
		http.Error(w, "Failed to bulk archive tasks", http.StatusInternalServerError)
		return
	}

	log.Printf("Bulk archived %d tasks", count)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "archived", "count": count})
}

// splitByComma splits a string by comma
func splitByComma(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// Queue handlers
func getQueueMessages(w http.ResponseWriter, r *http.Request) {
	messages := queueService.GetMessages()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"messages": messages})
}

func simulateTask(w http.ResponseWriter, r *http.Request) {
	// Simulation feature removed - task assignment is now handled by manager
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "not_implemented",
		"message": "Task simulation removed - use manager for assignment",
	})
}

// handleGitHubWebhook processes incoming GitHub webhook events.
// It verifies the HMAC-SHA256 signature, determines the event type,
// and dispatches to the appropriate handler.
// getGitHubSummary returns aggregated GitHub data for configured repos
func getGitHubSummary(w http.ResponseWriter, r *http.Request) {
	if githubAPIService == nil {
		http.Error(w, `{"error":"GitHub API service not initialized"}`, http.StatusInternalServerError)
		return
	}

	summary, err := githubAPIService.GetSummary()
	if err != nil {
		log.Printf("[GitHub API] Error fetching summary: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// refreshGitHubSummary clears cache and returns fresh GitHub data
func refreshGitHubSummary(w http.ResponseWriter, r *http.Request) {
	if githubAPIService == nil {
		http.Error(w, `{"error":"GitHub API service not initialized"}`, http.StatusInternalServerError)
		return
	}

	summary, err := githubAPIService.RefreshSummary()
	if err != nil {
		log.Printf("[GitHub API] Error refreshing summary: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if githubWebhookService == nil {
		http.Error(w, `{"error":"GitHub webhook service not initialized"}`, http.StatusInternalServerError)
		return
	}

	// Verify signature
	body, valid := githubWebhookService.VerifySignature(r)
	if !valid {
		log.Println("[GitHub Webhook] Invalid signature, rejecting request")
		http.Error(w, `{"error":"invalid signature"}`, http.StatusUnauthorized)
		return
	}

	// Get event type
	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		http.Error(w, `{"error":"missing X-GitHub-Event header"}`, http.StatusBadRequest)
		return
	}

	deliveryID := r.Header.Get("X-GitHub-Delivery")
	log.Printf("[GitHub Webhook] Received event=%s delivery=%s", event, deliveryID)

	// Process the event
	result, err := githubWebhookService.HandleEvent(event, body)
	if err != nil {
		log.Printf("[GitHub Webhook] Error processing event %s: %v", event, err)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// System status handler - checks tmux session directly
func getSystemStatus(w http.ResponseWriter, r *http.Request) {
	status := make(map[string]interface{})

	// Check tmux session directly
	tmuxRunning := false
	agentPanes := 0
	claudeProcesses := 0

	cmd := exec.Command("tmux", "list-panes", "-t", tmuxDefaultSessionName, "-F", "#{pane_current_command}")
	output, err := cmd.Output()
	if err == nil {
		tmuxRunning = true
		lines := splitLines(string(output))
		for _, line := range lines {
			if line != "" {
				agentPanes++
				if line == "claude" {
					claudeProcesses++
				}
			}
		}
	}

	// Auto-expire old pending tasks (older than 24 hours)
	if taskRepo != nil {
		if expired, err := taskRepo.CloseOldPendingTasks(24); err == nil && expired > 0 {
			log.Printf("Auto-expired %d old pending tasks", expired)
		}
	}

	// Get active task count from database (excludes completed/cancelled/expired and test commands)
	activeTasks := 0
	if taskRepo != nil {
		if count, err := taskRepo.CountActive(); err == nil {
			activeTasks = count
		}
	}

	// Determine overall status
	overallStatus := "stopped"
	if tmuxRunning {
		if claudeProcesses > 0 {
			overallStatus = "running"
		} else {
			overallStatus = "partial"
		}
	}

	status["tmux_session"] = tmuxRunning
	status["agent_panes"] = agentPanes
	status["claude_processes"] = claudeProcesses
	status["pending_tasks"] = activeTasks
	status["active_tasks"] = activeTasks
	status["status"] = overallStatus
	status["database"] = db != nil

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Agent message sending
const (
	maxDirectMessageLen    = 16000 // Messages longer than this go to file
	tmuxDefaultSessionName = "manager"
)

// tmuxMutex serializes tmux send-keys operations to prevent race conditions
// when multiple goroutines send messages to tmux panes concurrently
var tmuxMutex sync.Mutex

// tmuxTargetFormat is the tmux target format determined at startup based on OS.
// macOS uses "session.pane", Linux uses "session:window.pane".
var tmuxTargetFormat string

func init() {
	if runtime.GOOS == "darwin" {
		tmuxTargetFormat = "%s.%d" // macOS style: session.pane
	} else {
		tmuxTargetFormat = "%s:0.%d" // Linux style: session:window.pane
	}
	log.Printf("tmux target format: %q (OS: %s)", tmuxTargetFormat, runtime.GOOS)
}

func getManagerNames() []string {
	if appConfig != nil && appConfig.Agents != nil && appConfig.Agents.AgentTeams != nil {
		if appConfig.Agents.AgentTeams.TeamName != "" {
			return []string{appConfig.Agents.AgentTeams.TeamName}
		}
	}
	return []string{tmuxDefaultSessionName}
}

// getAgentPaneIndex returns the tmux pane index for an agent from config
func getAgentPaneIndex(agentID string) (int, bool) {
	if appConfig == nil {
		return -1, false
	}
	pane := appConfig.GetWorkerPaneIndex(agentID)
	if pane < 0 {
		return -1, false
	}
	return pane, true
}

type SendMessageRequest struct {
	From    string `json:"from"`    // sender agent ID
	Message string `json:"message"` // message content
}

type SendMessageResponse struct {
	Success  bool   `json:"success"`
	Method   string `json:"method"` // "direct" or "file"
	FilePath string `json:"file_path,omitempty"`
	Error    string `json:"error,omitempty"`
}

func sendAgentMessage(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")

	// Validate target agent using config
	pane, ok := getAgentPaneIndex(agentID)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SendMessageResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown agent: %s", agentID),
		})
		return
	}

	// Parse request body
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SendMessageResponse{
			Success: false,
			Error:   "invalid request body",
		})
		return
	}

	if req.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SendMessageResponse{
			Success: false,
			Error:   "message is required",
		})
		return
	}

	var response SendMessageResponse
	var messageToSend string

	// Decide whether to send directly or via file
	if len(req.Message) <= maxDirectMessageLen {
		// Direct send
		messageToSend = req.Message
		response.Method = "direct"
	} else {
		// Write to file and send file path
		filePath, err := writeMessageToFile(req.From, agentID, req.Message)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(SendMessageResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to write message file: %v", err),
			})
			return
		}
		messageToSend = fmt.Sprintf("%s からのメッセージがあります。%s を読んで対応してください。", req.From, filePath)
		response.Method = "file"
		response.FilePath = filePath
	}

	// Send via tmux send-keys
	if err := sendTmuxMessage(pane, messageToSend); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SendMessageResponse{
			Success: false,
			Method:  response.Method,
			Error:   fmt.Sprintf("failed to send tmux message: %v", err),
		})
		return
	}

	// Log the message
	if queueService != nil {
		// Add to communication log (truncate long messages)
		logContent := req.Message
		if len(logContent) > 200 {
			logContent = logContent[:200] + "..."
		}
		// Note: We'd need to add a method to queue service for this
	}

	response.Success = true
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// writeMessageToFile writes a long message to a temp file and returns the path
func writeMessageToFile(from, to, message string) (string, error) {
	// Create temp directory if needed
	tmpDir := "/tmp/claude-hub-messages"
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	// Create file with timestamp
	filename := fmt.Sprintf("%s/msg-%s-to-%s-%d.txt", tmpDir, from, to, time.Now().UnixNano())
	content := fmt.Sprintf("From: %s\nTo: %s\nTime: %s\n\n%s", from, to, time.Now().Format(time.RFC3339), message)

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return "", err
	}

	return filename, nil
}

// sendTmuxMessage sends a message to a tmux pane via send-keys using the default session.
// This is the backward-compatible wrapper around sendTmuxMessageToSession.
func sendTmuxMessage(pane int, message string) error {
	return sendTmuxMessageToSession(tmuxDefaultSessionName, pane, message)
}

func hasTmuxSession(session string) bool {
	cmd := execCommand("tmux", "has-session", "-t", session)
	return cmd.Run() == nil
}

// sendTmuxMessageToSession sends a message to a specific tmux session and pane.
// Uses mutex to prevent race conditions when called from multiple goroutines,
// and adds a small delay between message and Enter to ensure reliable delivery.
// The tmux target format is determined once at startup based on runtime.GOOS.
// Codex sessions automatically use "Enter" instead of "C-m" (see #836).
func sendTmuxMessageToSession(session string, pane int, message string) error {
	enterKey := "C-m"
	if session == "codex" {
		enterKey = "Enter"
	}
	return sendTmuxMessageToSessionWithEnterKey(session, pane, message, enterKey)
}

// sendTmuxMessageToSessionWithEnterKey sends a message with a configurable enter key.
// Codex TUI requires "Enter" instead of "C-m".
func sendTmuxMessageToSessionWithEnterKey(session string, pane int, message string, enterKey string) error {
	tmuxMutex.Lock()
	defer tmuxMutex.Unlock()

	target := fmt.Sprintf(tmuxTargetFormat, session, pane)

	// Step 1: Send the message text with -l (literal) flag
	cmd := execCommand("tmux", "send-keys", "-t", target, "-l", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("send message failed for %s: %w (output: %s)", target, err, string(output))
	}

	// Step 2: Pause to ensure tmux processes the message buffer
	// 50ms was too short for long messages — Claude Code could miss partial input
	time.Sleep(1 * time.Second)

	// Step 3: Send Enter key (configurable: "C-m" for Claude Code, "Enter" for Codex)
	cmd = execCommand("tmux", "send-keys", "-t", target, enterKey)
	output, err = cmd.CombinedOutput()
	if err != nil {
		// Clear input buffer to prevent stale message in the pane
		clearCmd := execCommand("tmux", "send-keys", "-t", target, "C-u")
		clearCmd.Run() // Ignore error - best effort cleanup
		return fmt.Errorf("send Enter failed for %s: %w (output: %s)", target, err, string(output))
	}

	log.Printf("Successfully sent message to tmux session=%s pane=%d (target: %s)", session, pane, target)
	return nil
}

// resolveManagerSession resolves a manager name to its tmux session name.
func resolveManagerSession(templateName string) string {
	if appConfig != nil && appConfig.Agents != nil {
		isMultiSession := appConfig.Agents.Mode == "agent_teams" &&
			appConfig.Agents.AgentTeams != nil && appConfig.Agents.AgentTeams.Enabled
		if isMultiSession && templateName != "" {
			return templateName
		}
	}
	return tmuxDefaultSessionName
}

// resolveManagerPane returns the tmux pane index for a manager.
// In Agent Teams mode (multi-session), each manager has its own session with pane 0.
// In single-session mode, the manager pane from config is used.
func resolveManagerPane() int {
	if appConfig != nil && appConfig.Agents != nil {
		isMultiSession := appConfig.Agents.Mode == "agent_teams" &&
			appConfig.Agents.AgentTeams != nil && appConfig.Agents.AgentTeams.Enabled
		if !isMultiSession && appConfig.Agents.Manager.Enabled {
			return appConfig.Agents.Manager.Pane
		}
	}
	return 0
}

// sendManagerMessage sends a message to a Manager's tmux session.
func sendManagerMessage(templateName string, message string) error {
	touchManagerActivity(templateName)
	session := resolveManagerSession(templateName)
	pane := resolveManagerPane()
	enterKey := resolveEnterKey(templateName)
	return sendTmuxMessageToSessionWithEnterKey(session, pane, message, enterKey)
}

// resolveEnterKey returns the enter key for a team lead.
// Codex TUI requires "Enter" instead of "C-m".
func resolveEnterKey(templateName string) string {
	return "C-m"
}

// Backward compatibility aliases
var (
	resolveTeamLeadSession = resolveManagerSession
	resolveTeamLeadPane    = resolveManagerPane
	sendTeamLeadMessage    = sendManagerMessage
)

// isClaudeProcessAlive checks if a Claude process is running inside the given tmux session.
// Uses multiple detection strategies for robustness.
// This is a best-effort check - false negatives are possible during process transitions.
func isClaudeProcessAlive(session string) bool {
	// Get pane PID from tmux
	pidCmd := execCommand("tmux", "list-panes", "-t", session, "-F", "#{pane_pid}")
	pidOutput, err := pidCmd.Output()
	if err != nil {
		return false
	}

	pids := strings.Split(strings.TrimSpace(string(pidOutput)), "\n")
	for _, pid := range pids {
		pid = strings.TrimSpace(pid)
		if pid == "" {
			continue
		}
		// Strategy 1: Check direct child processes via pgrep
		pgrepCmd := execCommand("pgrep", "-P", pid, "-f", "claude")
		if pgrepCmd.Run() == nil {
			return true
		}
		// Strategy 2: Walk the full descendant tree (handles deeper nesting)
		psCmd := execCommand("bash", "-c",
			fmt.Sprintf("ps -eo pid,ppid,comm --no-headers | awk 'BEGIN{p[\"%s\"]=1} p[$2]{p[$1]=1; if($3~/claude/)found=1} END{exit !found}'", pid))
		if psCmd.Run() == nil {
			return true
		}
	}
	return false
}

// isTmuxSessionExists checks if a tmux session exists (session-level only, no process check).
func isTmuxSessionExists(session string) bool {
	cmd := execCommand("tmux", "has-session", "-t", session)
	return cmd.Run() == nil
}

// isManagerRunning checks if a Manager's tmux session is alive.
// NOTE: This intentionally checks tmux session only (not Claude process).
func isManagerRunning(templateName string) bool {
	session := resolveManagerSession(templateName)
	return isTmuxSessionExists(session)
}

// isTeamLeadRunning is a backward compatibility alias for isManagerRunning
var isTeamLeadRunning = isManagerRunning

// autoStartManager starts a Manager's tmux session on demand via claude-hub.sh.
func autoStartManager(templateName string) error {
	session := resolveManagerSession(templateName)

	// Double-check session is not already running
	checkCmd := execCommand("tmux", "has-session", "-t", session)
	if checkCmd.Run() == nil {
		return nil // already running
	}

	hubScript := findHubScript()
	if hubScript == "" {
		return fmt.Errorf("claude-hub.sh script not found")
	}

	teammateCount := 2 // default
	bashCmd := fmt.Sprintf(
		"cd %s && %s start-manager %s %d > /tmp/claude-hub-start-manager-%s.log 2>&1",
		filepath.Dir(hubScript),
		hubScript,
		templateName,
		teammateCount,
		templateName,
	)

	cmd := execCommand("bash", "-c", bashCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude-hub.sh start-manager %s failed: %w", templateName, err)
	}

	log.Printf("Auto-started Manager %s (session: %s, teammates: %d)", templateName, session, teammateCount)
	touchManagerActivity(templateName)

	// Post-startup health verification (async)
	go func() {
		healthy, reason := verifyStartupHealth(templateName, 60*time.Second)
		if !healthy {
			log.Printf("Auto-start failure detected for Manager %s: %s", templateName, reason)
			notifyStartupFailure("Manager", templateName, reason)
		}
	}()

	return nil
}

// autoStartTeamLead is a backward compatibility alias for autoStartManager
var autoStartTeamLead = autoStartManager

// waitForManagerSession polls until the Manager's tmux session is running
// or the timeout is reached. Returns true if session became ready.
func waitForManagerSession(templateName string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isManagerRunning(templateName) {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

// verifyStartupHealth checks if a Manager's tmux session started successfully
// and has a Claude Code process running. Returns an error description if unhealthy.
func verifyStartupHealth(templateName string, timeout time.Duration) (bool, string) {
	session := resolveManagerSession(templateName)
	startTime := time.Now()

	// Phase 1: Wait for tmux session to exist
	if !waitForManagerSession(templateName, timeout) {
		return false, fmt.Sprintf("tmuxセッション '%s' が%v以内に起動しませんでした", session, timeout)
	}

	// Phase 2: Check that a Claude process is running inside the session
	remaining := timeout - time.Since(startTime)
	if remaining < 5*time.Second {
		remaining = 5 * time.Second
	}
	deadline := time.Now().Add(remaining)
	for time.Now().Before(deadline) {
		// Check if claude process exists in the session
		checkCmd := execCommand("bash", "-c",
			fmt.Sprintf("tmux list-panes -t %s -F '#{pane_pid}' 2>/dev/null | head -1 | xargs -I{} bash -c 'ps --ppid {} -o comm= 2>/dev/null | grep -q claude'", session))
		if checkCmd.Run() == nil {
			return true, ""
		}
		time.Sleep(3 * time.Second)
	}

	// Session exists but no Claude process detected - could be a hang
	return false, fmt.Sprintf("tmuxセッション '%s' は存在しますが、Claudeプロセスが検出されません（ハング可能性）", session)
}

// notifyStartupFailure logs a startup failure.
func notifyStartupFailure(agentType, agentName, reason string) {
	log.Printf("Startup failure detected: %s %s - %s", agentType, agentName, reason)
}

// --- Manager idle shutdown ---

// touchManagerActivity records that a Manager had activity (message sent/received).
func touchManagerActivity(templateName string) {
	managerLastActivityLock.Lock()
	defer managerLastActivityLock.Unlock()
	managerLastActivity[templateName] = time.Now()
}

// clearManagerActivity removes a Manager's activity record (after shutdown).
func clearManagerActivity(templateName string) {
	managerLastActivityLock.Lock()
	defer managerLastActivityLock.Unlock()
	delete(managerLastActivity, templateName)
}

// getTmuxSessionActivity returns the last activity time of a tmux session
// by reading the session_activity format variable (epoch seconds).
func getTmuxSessionActivity(session string) time.Time {
	cmd := execCommand("tmux", "display-message", "-t", session, "-p", "#{session_activity}")
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}
	}
	epoch, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil || epoch == 0 {
		return time.Time{}
	}
	return time.Unix(epoch, 0)
}

// getManagerIdleDuration calculates how long a Manager has been idle
// using both Go API activity tracking and tmux session_activity.
func getManagerIdleDuration(name, session string) time.Duration {
	now := time.Now()

	// Go API last activity
	managerLastActivityLock.RLock()
	lastAPI := managerLastActivity[name]
	managerLastActivityLock.RUnlock()

	// tmux session_activity (keyboard/mouse)
	lastTmux := getTmuxSessionActivity(session)

	// Use the more recent of the two
	lastActive := lastAPI
	if lastTmux.After(lastActive) {
		lastActive = lastTmux
	}

	if lastActive.IsZero() {
		// No activity recorded; treat as idle since epoch (very long)
		return now.Sub(time.Time{})
	}

	return now.Sub(lastActive)
}

// isManagerExcludedFromShutdown checks if a Manager should be excluded from auto-shutdown.
func isManagerExcludedFromShutdown(templateName string, excludeList []string) bool {
	for _, excluded := range excludeList {
		if excluded == templateName {
			return true
		}
	}

	// Check if manager has active tasks (status = busy)
	agentStatusLock.RLock()
	defer agentStatusLock.RUnlock()
	if status, ok := agentStatuses[templateName]; ok {
		if status.Status == "busy" && status.CurrentTask != "" {
			return true
		}
	}

	return false
}

// startIdleShutdownWatcher starts a background goroutine that periodically checks
// for idle Manager sessions and shuts them down to save resources.
func startIdleShutdownWatcher(idleShutdownCfg *config.IdleShutdownConfig) {
	if idleShutdownCfg == nil || !idleShutdownCfg.Enabled {
		log.Println("Idle shutdown watcher: disabled")
		return
	}

	checkInterval := time.Duration(idleShutdownCfg.CheckIntervalMinutes) * time.Minute
	if checkInterval <= 0 {
		checkInterval = 5 * time.Minute
	}
	idleTimeout := time.Duration(idleShutdownCfg.IdleTimeoutMinutes) * time.Minute
	if idleTimeout <= 0 {
		idleTimeout = 30 * time.Minute
	}

	log.Printf("Idle shutdown watcher: started (check=%v, timeout=%v, exclude=%v)",
		checkInterval, idleTimeout, idleShutdownCfg.Exclude)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		checkAndShutdownIdleManagers(idleTimeout, idleShutdownCfg)
	}
}

// checkAndShutdownIdleManagers checks all configured Managers and shuts down idle ones.
func checkAndShutdownIdleManagers(timeout time.Duration, cfg *config.IdleShutdownConfig) {
	for _, name := range getManagerNames() {
		// Skip excluded managers
		if isManagerExcludedFromShutdown(name, cfg.Exclude) {
			continue
		}

		session := resolveManagerSession(name)

		// Skip if not running
		checkCmd := execCommand("tmux", "has-session", "-t", session)
		if checkCmd.Run() != nil {
			continue
		}

		// Calculate idle duration
		idleDuration := getManagerIdleDuration(name, session)
		if idleDuration < timeout {
			continue
		}

		log.Printf("Auto-shutdown: Manager %s idle for %v (threshold: %v)", name, idleDuration.Truncate(time.Second), timeout)

		// Kill the tmux session
		killCmd := execCommand("tmux", "kill-session", "-t", session)
		if err := killCmd.Run(); err != nil {
			log.Printf("Failed to auto-shutdown Manager %s (session: %s): %v", name, session, err)
			continue
		}

		log.Printf("Auto-shutdown completed: Manager %s (session: %s)", name, session)

		// Clear activity record
		clearManagerActivity(name)
	}
}

// --- Memory Monitor (Issue #215) ---

// startMemoryMonitor starts a background goroutine that periodically checks
// Manager process memory usage and kills/restarts processes exceeding the threshold.
func startMemoryMonitor(cfg *config.MemoryMonitorConfig) {
	if cfg == nil || !cfg.Enabled {
		log.Println("Memory monitor: disabled")
		return
	}

	checkInterval := time.Duration(cfg.CheckIntervalSeconds) * time.Second
	if checkInterval <= 0 {
		checkInterval = 30 * time.Second
	}
	thresholdMB := cfg.ThresholdMB
	if thresholdMB <= 0 {
		thresholdMB = 1500
	}
	gracePeriod := time.Duration(cfg.GracePeriodSeconds) * time.Second
	if gracePeriod <= 0 {
		gracePeriod = 30 * time.Second
	}

	log.Printf("Memory monitor: started (interval=%v, threshold=%dMB, grace=%v)",
		checkInterval, thresholdMB, gracePeriod)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		checkManagerMemoryUsage(thresholdMB, gracePeriod, cfg)
	}
}

// checkManagerMemoryUsage checks all running Managers' memory usage
// and kills/restarts those exceeding the threshold.
func checkManagerMemoryUsage(thresholdMB int, gracePeriod time.Duration, cfg *config.MemoryMonitorConfig) {
	for _, name := range getManagerNames() {
		session := resolveManagerSession(name)

		// Skip if not running
		if !isManagerRunning(name) {
			continue
		}

		memMB, err := getSessionMemoryMB(session)
		if err != nil {
			log.Printf("Memory monitor: failed to get memory for %s: %v", name, err)
			continue
		}

		// Determine if Manager should be killed based on threshold type
		shouldKill := false
		var killReason string

		thresholdType := cfg.ThresholdType
		if thresholdType == "" {
			thresholdType = "fixed" // Default to fixed for backward compatibility
		}

		if thresholdType == "usage_rate" {
			// Usage rate based monitoring
			totalMB, availableMB, err := getSystemMemoryInfo()
			if err != nil {
				log.Printf("Memory monitor: failed to get system memory info: %v", err)
				continue
			}

			usedMB := totalMB - availableMB
			usageRate := float64(usedMB) / float64(totalMB)

			minFreeMemoryMB := cfg.MinFreeMemoryMB
			if minFreeMemoryMB <= 0 {
				minFreeMemoryMB = 1000
			}

			systemUsageRate := cfg.SystemMemoryUsageRate
			if systemUsageRate <= 0 {
				systemUsageRate = 0.8
			}

			if usageRate > systemUsageRate && memMB > 500 {
				shouldKill = true
				killReason = fmt.Sprintf("System memory usage %.1f%% > %.1f%% and Manager using %dMB",
					usageRate*100, systemUsageRate*100, memMB)
			} else if availableMB < minFreeMemoryMB {
				shouldKill = true
				killReason = fmt.Sprintf("Available memory %dMB < %dMB (min free)", availableMB, minFreeMemoryMB)
			}
		} else {
			// Fixed threshold (legacy)
			if memMB >= thresholdMB {
				shouldKill = true
				killReason = fmt.Sprintf("Manager memory %dMB >= %dMB (threshold)", memMB, thresholdMB)
			}
		}

		if !shouldKill {
			continue
		}

		log.Printf("Memory monitor: %s", killReason)

		// Wait grace period
		log.Printf("Memory monitor: waiting %v grace period before killing %s", gracePeriod, name)
		time.Sleep(gracePeriod)

		// Re-check: session may have been stopped during grace period
		if !isManagerRunning(name) {
			log.Printf("Memory monitor: %s already stopped during grace period", name)
			continue
		}

		// Kill the tmux session
		killCmd := execCommand("tmux", "kill-session", "-t", session)
		if err := killCmd.Run(); err != nil {
			log.Printf("Memory monitor: failed to kill %s (session: %s): %v", name, session, err)
			continue
		}

		log.Printf("Memory monitor: killed Manager %s (session: %s, memory: %dMB)", name, session, memMB)
		clearManagerActivity(name)

		// Auto-restart
		time.Sleep(3 * time.Second) // Brief pause before restart
		if err := autoStartManager(name); err != nil {
			log.Printf("Memory monitor: failed to restart %s: %v", name, err)
			continue
		}

		log.Printf("Memory monitor: restarted Manager %s", name)
	}
}

// --- Claude process health monitor ---

var (
	claudeHealthFailCounts   = make(map[string]int)
	claudeHealthFailCountsMu sync.Mutex
)

const (
	claudeHealthCheckInterval = 60 * time.Second
	claudeHealthFailThreshold = 3 // consecutive failures before restart
)

// startClaudeProcessHealthMonitor periodically checks that Claude processes
// are alive inside running tmux sessions. If a session has a dead Claude
// process for multiple consecutive checks, it kills the stale session and restarts.
func startClaudeProcessHealthMonitor() {
	log.Printf("Claude process health monitor: started (interval=%v, threshold=%d consecutive failures)",
		claudeHealthCheckInterval, claudeHealthFailThreshold)

	ticker := time.NewTicker(claudeHealthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		checkClaudeProcessHealth()
	}
}

func checkClaudeProcessHealth() {
	// Check all Managers
	for _, name := range getManagerNames() {
		session := resolveManagerSession(name)
		if !isTmuxSessionExists(session) {
			resetClaudeHealthCount(session)
			continue
		}
		updateClaudeHealthStatus(session, name, isClaudeProcessAlive(session))
	}
}

func updateClaudeHealthStatus(session, name string, alive bool) {
	claudeHealthFailCountsMu.Lock()
	defer claudeHealthFailCountsMu.Unlock()

	if alive {
		if claudeHealthFailCounts[session] > 0 {
			log.Printf("Claude process health: %s recovered (was at %d failures)", name, claudeHealthFailCounts[session])
		}
		claudeHealthFailCounts[session] = 0
		return
	}

	claudeHealthFailCounts[session]++
	count := claudeHealthFailCounts[session]
	log.Printf("Claude process health: %s - no Claude process detected (%d/%d)", name, count, claudeHealthFailThreshold)

	if count >= claudeHealthFailThreshold {
		claudeHealthFailCounts[session] = 0
		go handleStaleSession(session, name)
	}
}

func resetClaudeHealthCount(session string) {
	claudeHealthFailCountsMu.Lock()
	delete(claudeHealthFailCounts, session)
	claudeHealthFailCountsMu.Unlock()
}

func handleStaleSession(session, name string) {
	log.Printf("Stale session detected: %s (%s) - Claude process dead for %d+ consecutive checks", name, session, claudeHealthFailThreshold)

	// Kill stale session
	killCmd := execCommand("tmux", "kill-session", "-t", session)
	if err := killCmd.Run(); err != nil {
		log.Printf("Failed to kill stale session %s: %v", session, err)
		return
	}

	// Restart
	err := autoStartManager(name)
	if err != nil {
		log.Printf("Failed to auto-restart %s after stale detection: %v", name, err)
	} else {
		log.Printf("Auto-restarted %s (%s) after stale session detection", name, session)
	}
}

// getSessionMemoryMB returns the total RSS memory usage (in MB) of all Claude processes
// running inside a tmux session. It uses pgrep to find processes associated with the
// session's shell PID and reads RSS from /proc or ps.
func getSessionMemoryMB(session string) (int, error) {
	// Get the PID of the tmux session's initial process
	cmd := execCommand("tmux", "list-panes", "-t", session, "-F", "#{pane_pid}")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get pane PIDs for session %s: %w", session, err)
	}

	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(pids) == 0 || pids[0] == "" {
		return 0, fmt.Errorf("no pane PIDs found for session %s", session)
	}

	// For each pane PID, get the total RSS of its process tree
	totalKB := 0
	for _, pid := range pids {
		pid = strings.TrimSpace(pid)
		if pid == "" {
			continue
		}

		// Use ps to get RSS of the process and all descendants
		psCmd := execCommand("ps", "--ppid", pid, "-o", "rss=", "--no-headers")
		psOutput, err := psCmd.Output()
		if err != nil {
			// No children or ps failed - try the PID itself
			psCmd = execCommand("ps", "-p", pid, "-o", "rss=", "--no-headers")
			psOutput, err = psCmd.Output()
			if err != nil {
				continue
			}
		}

		// Also include the parent process itself
		parentCmd := execCommand("ps", "-p", pid, "-o", "rss=", "--no-headers")
		parentOutput, _ := parentCmd.Output()

		// Sum all RSS values
		for _, line := range strings.Split(strings.TrimSpace(string(psOutput)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			kb, err := strconv.Atoi(line)
			if err == nil {
				totalKB += kb
			}
		}
		for _, line := range strings.Split(strings.TrimSpace(string(parentOutput)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			kb, err := strconv.Atoi(line)
			if err == nil {
				totalKB += kb
			}
		}
	}

	return totalKB / 1024, nil
}

// getSystemMemoryInfo returns total and available memory in MB from /proc/meminfo.
func getSystemMemoryInfo() (totalMB int, availableMB int, err error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read /proc/meminfo: %w", err)
	}

	var total, available int64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		if fields[0] == "MemTotal:" {
			if val, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				total = val
			}
		} else if fields[0] == "MemAvailable:" {
			if val, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				available = val
			}
		}
	}

	if total == 0 {
		return 0, 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
	}
	if available == 0 {
		return 0, 0, fmt.Errorf("MemAvailable not found in /proc/meminfo")
	}

	return int(total / 1024), int(available / 1024), nil
}

// --- Manager API handlers ---

// listManagers returns all configured managers/templates with their session info
func listManagers(w http.ResponseWriter, r *http.Request) {
	type ManagerInfo struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		TmuxSession string   `json:"tmux_session"`
		Repos       []string `json:"repos"`
		Model       string   `json:"model,omitempty"`
	}

	// Get default model from config
	defaultModel := ""
	if appConfig != nil && appConfig.Agents != nil && appConfig.Agents.AgentTeams != nil {
		defaultModel = appConfig.Agents.AgentTeams.TeamLead.Model
	}

	var managers []ManagerInfo
	for _, name := range getManagerNames() {
		managers = append(managers, ManagerInfo{
			Name:        name,
			Description: "",
			TmuxSession: resolveManagerSession(name),
			Repos:       []string{},
			Model:       defaultModel,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"team_leads":      managers, // backward compat JSON key
		"default_session": tmuxDefaultSessionName,
	})
}

// sendManagerMessageHandler handles POST /api/v1/teams/{teamLead}/message
func sendManagerMessageHandler(w http.ResponseWriter, r *http.Request) {
	teamLead := chi.URLParam(r, "teamLead")

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "invalid request body",
		})
		return
	}

	if req.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "message is required",
		})
		return
	}

	session := resolveManagerSession(teamLead)
	autoStarted := false

	// Long message handling: write to file if message exceeds threshold
	var messageToSend string
	var method string
	var filePath string

	if len(req.Message) <= maxDirectMessageLen {
		messageToSend = req.Message
		method = "direct"
	} else {
		var err error
		filePath, err = writeMessageToFile(req.From, teamLead, req.Message)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("failed to write message file: %v", err),
			})
			return
		}
		messageToSend = fmt.Sprintf("%s からのメッセージがあります。%s を読んで対応してください。", req.From, filePath)
		method = "file"
	}

	pane := resolveManagerPane()
	enterKey := resolveEnterKey(teamLead)
	if err := sendTmuxMessageToSessionWithEnterKey(session, pane, messageToSend, enterKey); err != nil {
		// Check if session exists
		if !isManagerRunning(teamLead) {
			log.Printf("Manager %s session not found, attempting auto-start...", teamLead)

			// Mutex to prevent concurrent auto-starts for the same manager
			managerAutoStartMu.Lock()
			if managerAutoStarting[teamLead] {
				managerAutoStartMu.Unlock()
				log.Printf("Manager %s is already being auto-started, waiting...", teamLead)
				// Wait for Claude process to be ready (not just tmux session)
				verifyStartupHealth(teamLead, 60*time.Second)
			} else {
				managerAutoStarting[teamLead] = true
				managerAutoStartMu.Unlock()

				startErr := autoStartManager(teamLead)

				managerAutoStartMu.Lock()
				delete(managerAutoStarting, teamLead)
				managerAutoStartMu.Unlock()

				if startErr != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   fmt.Sprintf("session not running and auto-start failed for %s: %v", teamLead, startErr),
					})
					return
				}

				// Wait for Claude process to be ready (not just tmux session)
				if healthy, reason := verifyStartupHealth(teamLead, 60*time.Second); !healthy {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   fmt.Sprintf("auto-started %s but not ready: %s", teamLead, reason),
					})
					return
				}
			}

			autoStarted = true

			// Retry sending the message (using the same long-text-handled message)
			if retryErr := sendTmuxMessageToSessionWithEnterKey(session, pane, messageToSend, enterKey); retryErr != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success":      false,
					"auto_started": true,
					"error":        fmt.Sprintf("auto-started %s but message retry failed: %v", teamLead, retryErr),
				})
				return
			}
		} else {
			// Session exists but send failed for another reason
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("failed to send message to %s (session: %s): %v", teamLead, session, err),
			})
			return
		}
	}

	touchManagerActivity(teamLead)
	log.Printf("Sent message to manager %s (session: %s, auto_started: %v, method: %s)", teamLead, session, autoStarted, method)

	response := map[string]interface{}{
		"success":      true,
		"team_lead":    teamLead, // backward compat JSON key
		"tmux_session": session,
		"auto_started": autoStarted,
		"method":       method,
	}
	if filePath != "" {
		response["file_path"] = filePath
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// startManagerHandler handles POST /api/v1/teams/{teamLead}/start
func startManagerHandler(w http.ResponseWriter, r *http.Request) {
	teamLead := chi.URLParam(r, "teamLead")

	session := resolveManagerSession(teamLead)

	// Check if session already exists
	if isTmuxSessionExists(session) {
		if isClaudeProcessAlive(session) {
			// Session exists AND Claude is alive → truly running, return 409
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":              false,
				"error":                fmt.Sprintf("session %s is already running", session),
				"session":              session,
				"claude_process_alive": true,
			})
			return
		}
		// Session exists but Claude is dead → kill stale session and restart
		log.Printf("Stale tmux session detected for %s (session: %s) - killing before restart", teamLead, session)
		killCmd := execCommand("tmux", "kill-session", "-t", session)
		if err := killCmd.Run(); err != nil {
			log.Printf("Failed to kill stale session %s: %v", session, err)
		}
	}

	// Find claude-hub.sh script
	hubScript := findHubScript()
	if hubScript == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "claude-hub.sh script not found",
		})
		return
	}

	// Parse optional teammate count from request body
	teammateCount := 2 // default
	var reqBody struct {
		Teammates int `json:"teammate_count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil && reqBody.Teammates > 0 {
		teammateCount = reqBody.Teammates
		if teammateCount > 8 {
			teammateCount = 8
		}
	}

	// Execute claude-hub.sh start-manager in background
	bashCmd := fmt.Sprintf(
		"cd %s && nohup %s start-manager %s %d > /tmp/claude-hub-start-manager-%s.log 2>&1 &",
		filepath.Dir(hubScript),
		hubScript,
		teamLead,
		teammateCount,
		teamLead,
	)

	cmd := execCommand("bash", "-c", bashCmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Run(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to start manager: %v", err),
		})
		return
	}

	log.Printf("Started manager %s (session: %s, teammates: %d)", teamLead, session, teammateCount)
	touchManagerActivity(teamLead)

	// Post-startup health verification (async to avoid blocking the API response)
	go func() {
		healthy, reason := verifyStartupHealth(teamLead, 60*time.Second)
		if !healthy {
			log.Printf("Startup failure detected for Manager %s: %s", teamLead, reason)
			notifyStartupFailure("Manager", teamLead, reason)
		} else {
			log.Printf("Startup health verified for Manager %s", teamLead)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"team_lead":    teamLead, // backward compat JSON key
		"tmux_session": session,
		"teammates":    teammateCount,
		"message":      fmt.Sprintf("Manager %s starting in session %s", teamLead, session),
	})
}

// stopManagerHandler handles POST /api/v1/teams/{teamLead}/stop
func stopManagerHandler(w http.ResponseWriter, r *http.Request) {
	teamLead := chi.URLParam(r, "teamLead")

	session := resolveManagerSession(teamLead)

	// Check if session exists
	checkCmd := execCommand("tmux", "has-session", "-t", session)
	if checkCmd.Run() != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("session %s is not running", session),
			"session": session,
		})
		return
	}

	// Kill the tmux session
	killCmd := execCommand("tmux", "kill-session", "-t", session)
	if err := killCmd.Run(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to stop session %s: %v", session, err),
		})
		return
	}

	log.Printf("Stopped manager %s (session: %s)", teamLead, session)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"team_lead":    teamLead, // backward compat JSON key
		"tmux_session": session,
		"message":      fmt.Sprintf("Manager %s stopped (session %s killed)", teamLead, session),
	})
}

// getManagerStatusHandler handles GET /api/v1/teams/{teamLead}/status
func getManagerStatusHandler(w http.ResponseWriter, r *http.Request) {
	teamLead := chi.URLParam(r, "teamLead")
	session := resolveManagerSession(teamLead)

	// Check if session exists
	sessionExists := isTmuxSessionExists(session)

	// Check if Claude process is alive inside the session
	claudeAlive := false
	if sessionExists {
		claudeAlive = isClaudeProcessAlive(session)
	}

	running := sessionExists

	// Get pane count if session exists
	paneCount := 0
	if sessionExists {
		listCmd := execCommand("tmux", "list-panes", "-t", session)
		output, err := listCmd.Output()
		if err == nil {
			paneCount = len(strings.Split(strings.TrimSpace(string(output)), "\n"))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"team_lead":            teamLead,
		"tmux_session":         session,
		"running":              running,
		"claude_process_alive": claudeAlive,
		"panes":                paneCount,
	})
}

// findHubScript searches for claude-hub.sh in known paths
func findHubScript() string {
	searchPaths := []string{
		"./scripts/claude-hub.sh",
		"../scripts/claude-hub.sh",
	}

	// Also try relative to executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		searchPaths = append(searchPaths,
			filepath.Join(exeDir, "scripts", "claude-hub.sh"),
			filepath.Join(exeDir, "..", "scripts", "claude-hub.sh"),
		)
	}

	for _, p := range searchPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
	}
	return ""
}

// execCommand is a wrapper for exec.Command (allows mocking in tests)
var execCommand = func(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// Feature Request handlers
func listRequests(w http.ResponseWriter, r *http.Request) {
	if requestRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	// Pagination parameters
	page := 1
	perPage := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 100 {
			perPage = parsed
		}
	}
	offset := (page - 1) * perPage

	// Get total count
	total, err := requestRepo.Count()
	if err != nil {
		log.Printf("Failed to count requests: %v", err)
		total = 0
	}

	dbRequests, err := requestRepo.ListWithPagination(perPage, offset)
	if err != nil {
		log.Printf("Failed to list requests from DB: %v", err)
		http.Error(w, "Failed to list requests", http.StatusInternalServerError)
		return
	}

	requests := make([]map[string]interface{}, len(dbRequests))
	for i, req := range dbRequests {
		requests[i] = map[string]interface{}{
			"id":          req.ID,
			"title":       req.Title,
			"description": req.Description,
			"priority":    req.Priority,
			"status":      req.Status,
			"task_id":     req.TaskID,
			"created_at":  req.CreatedAt.Format(time.RFC3339),
			"updated_at":  req.UpdatedAt.Format(time.RFC3339),
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"requests": requests,
		"pagination": map[string]interface{}{
			"page":        page,
			"per_page":    perPage,
			"total":       total,
			"total_pages": (total + perPage - 1) / perPage,
		},
	})
}

type CreateRequestBody struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

func createRequest(w http.ResponseWriter, r *http.Request) {
	if requestRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	var body CreateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if body.Priority == "" {
		body.Priority = "medium"
	}

	// Create request in database
	reqID := fmt.Sprintf("REQ-%d", time.Now().UnixNano())
	var desc *string
	if body.Description != "" {
		desc = &body.Description
	}
	dbReq := &database.Request{
		ID:          reqID,
		Title:       body.Title,
		Description: desc,
		Priority:    body.Priority,
		Status:      "pending",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := requestRepo.Create(dbReq); err != nil {
		log.Printf("Failed to create request in DB: %v", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"id":          dbReq.ID,
		"title":       dbReq.Title,
		"description": dbReq.Description,
		"priority":    dbReq.Priority,
		"status":      dbReq.Status,
		"created_at":  dbReq.CreatedAt.Format(time.RFC3339),
		"updated_at":  dbReq.UpdatedAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func getRequest(w http.ResponseWriter, r *http.Request) {
	if requestRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	requestID := chi.URLParam(r, "requestID")
	dbReq, err := requestRepo.GetByID(requestID)
	if err != nil {
		log.Printf("Failed to get request from DB: %v", err)
		http.Error(w, "Failed to get request", http.StatusInternalServerError)
		return
	}
	if dbReq == nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	result := map[string]interface{}{
		"id":          dbReq.ID,
		"title":       dbReq.Title,
		"description": dbReq.Description,
		"priority":    dbReq.Priority,
		"status":      dbReq.Status,
		"task_id":     dbReq.TaskID,
		"created_at":  dbReq.CreatedAt.Format(time.RFC3339),
		"updated_at":  dbReq.UpdatedAt.Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// UpdateRequestBody represents a request to update a feature request
type UpdateRequestBody struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Priority    *string `json:"priority"`
}

func updateRequest(w http.ResponseWriter, r *http.Request) {
	if requestRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	requestID := chi.URLParam(r, "requestID")

	// Get existing request
	dbReq, err := requestRepo.GetByID(requestID)
	if err != nil {
		log.Printf("Failed to get request from DB: %v", err)
		http.Error(w, "Failed to get request", http.StatusInternalServerError)
		return
	}
	if dbReq == nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	// Only pending requests can be edited
	if dbReq.Status != "pending" {
		http.Error(w, "Only pending requests can be edited", http.StatusBadRequest)
		return
	}

	// Parse request body
	var body UpdateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Apply updates (use existing values if not provided)
	title := dbReq.Title
	if body.Title != nil && *body.Title != "" {
		title = *body.Title
	}
	description := dbReq.Description
	if body.Description != nil {
		description = body.Description
	}
	priority := dbReq.Priority
	if body.Priority != nil && *body.Priority != "" {
		// Validate priority
		validPriorities := map[string]bool{"low": true, "medium": true, "high": true}
		if !validPriorities[*body.Priority] {
			http.Error(w, "Invalid priority (must be low, medium, or high)", http.StatusBadRequest)
			return
		}
		priority = *body.Priority
	}

	// Update in database
	if err := requestRepo.Update(requestID, title, description, priority); err != nil {
		log.Printf("Failed to update request in DB: %v", err)
		http.Error(w, "Failed to update request", http.StatusInternalServerError)
		return
	}

	log.Printf("Request %s updated: title=%s, priority=%s", requestID, title, priority)

	// Return updated request
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":          requestID,
		"title":       title,
		"description": description,
		"priority":    priority,
		"status":      dbReq.Status,
		"task_id":     dbReq.TaskID,
		"updated_at":  time.Now().Format(time.RFC3339),
	})
}

func deleteRequest(w http.ResponseWriter, r *http.Request) {
	if requestRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	requestID := chi.URLParam(r, "requestID")
	if err := requestRepo.Delete(requestID); err != nil {
		log.Printf("Failed to delete request from DB: %v", err)
		http.Error(w, "Failed to delete request", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": requestID})
}

func executeRequest(w http.ResponseWriter, r *http.Request) {
	if requestRepo == nil || taskRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	requestID := chi.URLParam(r, "requestID")

	// Get request from database
	req, err := requestRepo.GetByID(requestID)
	if err != nil {
		log.Printf("Failed to get request from DB: %v", err)
		http.Error(w, "Failed to get request", http.StatusInternalServerError)
		return
	}
	if req == nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}
	if req.Status != "pending" {
		http.Error(w, fmt.Sprintf("Request already %s", req.Status), http.StatusBadRequest)
		return
	}

	// Create task from request
	taskID := fmt.Sprintf("TASK-%d", time.Now().UnixNano())
	description := req.Title
	if req.Description != nil && *req.Description != "" {
		description = fmt.Sprintf("%s: %s", req.Title, *req.Description)
	}

	dbTask := &database.Task{
		ID:          taskID,
		Type:        "feature",
		Priority:    req.Priority,
		Description: description,
		Status:      "pending",
		Source:      "request",
		CreatedAt:   time.Now(),
	}
	if err := taskRepo.Create(dbTask); err != nil {
		log.Printf("Failed to create task in DB: %v", err)
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// Update request status
	if err := requestRepo.UpdateStatus(requestID, "executed", &taskID); err != nil {
		log.Printf("Failed to update request status in DB: %v", err)
	}

	// Notify manager
	go func() {
		notification := fmt.Sprintf("リクエスト実行: %s - %s (優先度: %s)", taskID, description, req.Priority)
		if err := sendTmuxMessage(0, notification); err != nil {
			log.Printf("Failed to notify manager: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "executed",
		"request_id": requestID,
		"task": map[string]interface{}{
			"task_id":     dbTask.ID,
			"type":        dbTask.Type,
			"priority":    dbTask.Priority,
			"description": dbTask.Description,
			"status":      dbTask.Status,
			"source":      dbTask.Source,
			"created_at":  dbTask.CreatedAt.Format(time.RFC3339),
		},
	})
}

// getAgentsConfig returns the current agent configuration (Phase 7: Manager-Only)
func getAgentsConfig(w http.ResponseWriter, r *http.Request) {
	if appConfig == nil || appConfig.Services == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "configuration not loaded"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"architecture": "manager-only",
		"phase":        7,
		"manager": map[string]interface{}{
			"enabled":      true,
			"tmux_session": "manager",
			"working_dir":  "agents/manager",
		},
		"services": map[string]interface{}{
			"memory_monitor_enabled":       appConfig.Services.MemoryMonitor != nil && appConfig.Services.MemoryMonitor.Enabled,
			"startup_health_check_enabled": appConfig.Services.StartupHealthCheck != nil && appConfig.Services.StartupHealthCheck.Enabled,
			"ollama_enabled":               appConfig.Services.Ollama != nil && appConfig.Services.Ollama.Enabled,
		},
	})
}

// getLimitsConfig returns the current limits configuration
func getLimitsConfig(w http.ResponseWriter, r *http.Request) {
	if appConfig == nil || appConfig.Limits == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "configuration not loaded"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": map[string]interface{}{
			"max_tmp_agents": appConfig.Limits.Agents.MaxTmpAgents,
			"max_instances":  appConfig.Limits.Agents.MaxInstances,
		},
		"tasks": map[string]interface{}{
			"max_concurrent":       appConfig.Limits.Tasks.MaxConcurrent,
			"max_pending":          appConfig.Limits.Tasks.MaxPending,
			"max_duration_minutes": appConfig.Limits.Tasks.MaxDurationMinutes,
		},
		"rate_limits": map[string]interface{}{
			"task_create_per_minute":       appConfig.Limits.RateLimits.TaskCreatePerMinute,
			"message_per_minute_per_agent": appConfig.Limits.RateLimits.MessagePerMinutePerAgent,
			"discord_notify_per_minute":    appConfig.Limits.RateLimits.DiscordNotifyPerMinute,
			"api_requests_per_second":      appConfig.Limits.RateLimits.APIRequestsPerSecond,
		},
		"communication": map[string]interface{}{
			"pair_max_rounds":            appConfig.Limits.Communication.PairMaxRounds,
			"broadcast_interval_seconds": appConfig.Limits.Communication.BroadcastIntervalSeconds,
			"max_message_size":           appConfig.Limits.Communication.MaxMessageSize,
		},
		"resources": map[string]interface{}{
			"memory_warning_mb":    appConfig.Limits.Resources.MemoryWarningMB,
			"cpu_warning_percent":  appConfig.Limits.Resources.CPUWarningPercent,
			"disk_warning_percent": appConfig.Limits.Resources.DiskWarningPercent,
		},
	})
}

// Pending Question handlers

// listPendingQuestions returns all pending questions from Claude agents
func listPendingQuestions(w http.ResponseWriter, r *http.Request) {
	if questionRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	questions, err := questionRepo.ListPending(50)
	if err != nil {
		log.Printf("Failed to list pending questions: %v", err)
		http.Error(w, "Failed to list questions", http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, len(questions))
	for i, q := range questions {
		result[i] = map[string]interface{}{
			"id":            q.ID,
			"short_id":      q.ShortID,
			"agent_id":      q.AgentID,
			"session_id":    q.SessionID,
			"question_type": q.QuestionType,
			"question_text": q.QuestionText,
			"options":       q.Options,
			"status":        q.Status,
			"created_at":    q.CreatedAt.Format(time.RFC3339),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"questions": result})
}

// CreateQuestionRequest represents a request to create a pending question
type CreateQuestionRequest struct {
	AgentID      string   `json:"agent_id"`
	SessionID    *string  `json:"session_id,omitempty"`
	QuestionType string   `json:"question_type"` // "question" or "permission"
	QuestionText string   `json:"question_text"`
	Options      []string `json:"options,omitempty"`
}

// createPendingQuestion creates a new pending question and notifies Discord
func createPendingQuestion(w http.ResponseWriter, r *http.Request) {
	if questionRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	var req CreateQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" || req.QuestionText == "" {
		http.Error(w, "agent_id and question_text are required", http.StatusBadRequest)
		return
	}

	if req.QuestionType == "" {
		req.QuestionType = "question"
	}

	// Create question in database (short_id will be generated by Create)
	questionID := fmt.Sprintf("Q-%d", time.Now().UnixNano())
	dbQuestion := &database.PendingQuestion{
		ID:           questionID,
		AgentID:      req.AgentID,
		SessionID:    req.SessionID,
		QuestionType: req.QuestionType,
		QuestionText: req.QuestionText,
		Options:      req.Options,
		Status:       "pending",
		CreatedAt:    time.Now(),
	}

	if err := questionRepo.Create(dbQuestion); err != nil {
		log.Printf("Failed to create pending question: %v", err)
		http.Error(w, "Failed to create question", http.StatusInternalServerError)
		return
	}

	log.Printf("Created pending question %s (short: %s) from %s: %s", questionID, dbQuestion.ShortID, req.AgentID, req.QuestionText)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":            questionID,
		"short_id":      dbQuestion.ShortID,
		"agent_id":      req.AgentID,
		"question_type": req.QuestionType,
		"question_text": req.QuestionText,
		"options":       req.Options,
		"status":        "pending",
		"created_at":    dbQuestion.CreatedAt.Format(time.RFC3339),
	})
}

// getPendingQuestion returns a specific pending question
func getPendingQuestion(w http.ResponseWriter, r *http.Request) {
	if questionRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	questionID := chi.URLParam(r, "questionID")
	q, err := questionRepo.GetByID(questionID)
	if err != nil {
		log.Printf("Failed to get pending question: %v", err)
		http.Error(w, "Failed to get question", http.StatusInternalServerError)
		return
	}
	if q == nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}

	result := map[string]interface{}{
		"id":            q.ID,
		"short_id":      q.ShortID,
		"agent_id":      q.AgentID,
		"session_id":    q.SessionID,
		"question_type": q.QuestionType,
		"question_text": q.QuestionText,
		"options":       q.Options,
		"status":        q.Status,
		"answer":        q.Answer,
		"created_at":    q.CreatedAt.Format(time.RFC3339),
	}
	if q.AnsweredAt != nil {
		result["answered_at"] = q.AnsweredAt.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// AnswerQuestionRequest represents a request to answer a pending question
type AnswerQuestionRequest struct {
	Answer string `json:"answer"`
}

// answerPendingQuestion answers a pending question and sends the response to Claude
func answerPendingQuestion(w http.ResponseWriter, r *http.Request) {
	if questionRepo == nil {
		http.Error(w, "Database not initialized", http.StatusServiceUnavailable)
		return
	}

	questionID := chi.URLParam(r, "questionID")

	var req AnswerQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Answer == "" {
		http.Error(w, "answer is required", http.StatusBadRequest)
		return
	}

	// Get the question to find the agent
	q, err := questionRepo.GetByID(questionID)
	if err != nil {
		log.Printf("Failed to get pending question: %v", err)
		http.Error(w, "Failed to get question", http.StatusInternalServerError)
		return
	}
	if q == nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}
	if q.Status != "pending" {
		http.Error(w, "Question already answered", http.StatusBadRequest)
		return
	}

	// Send answer to Claude via tmux
	pane, ok := getAgentPaneIndex(q.AgentID)
	if !ok {
		http.Error(w, fmt.Sprintf("Unknown agent: %s", q.AgentID), http.StatusBadRequest)
		return
	}

	if err := sendTmuxMessage(pane, req.Answer); err != nil {
		log.Printf("Failed to send answer to tmux: %v", err)
		http.Error(w, "Failed to send answer to agent", http.StatusInternalServerError)
		return
	}

	// Mark question as answered
	if err := questionRepo.Answer(questionID, req.Answer); err != nil {
		log.Printf("Failed to mark question as answered: %v", err)
		// Don't return error since answer was sent successfully
	}

	log.Printf("Answered question %s from %s with: %s", questionID, q.AgentID, req.Answer)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "answered",
		"question_id": questionID,
		"agent_id":    q.AgentID,
		"answer":      req.Answer,
	})
}

// executeHubCommand executes claude-hub.sh with the given command (start, stop, restart, upgrade)
// This is used for remote control via Discord
func executeHubCommand(command string, args ...string) error {
	log.Printf("executeHubCommand called with command: %s", command)

	// Validate command to prevent injection
	validCommands := map[string]bool{
		"start":            true,
		"stop":             true,
		"restart":          true,
		"upgrade":          true,
		"status":           true,
		"recover":          true,
		"start-team":       true,
		"stop-team":        true,
	}
	if !validCommands[command] {
		log.Printf("executeHubCommand: invalid command: %s", command)
		return fmt.Errorf("invalid command: %s", command)
	}

	// Log current working directory
	if cwd, err := os.Getwd(); err == nil {
		log.Printf("executeHubCommand: current working directory: %s", cwd)
	}

	// Find claude-hub.sh path
	// Search order:
	// 1. ./scripts/claude-hub.sh (development environment)
	// 2. ./claude-hub.sh (deployment - same directory as working dir)
	// 3. {exeDir}/scripts/claude-hub.sh (based on executable location)
	// 4. {exeDir}/claude-hub.sh (deployment - same directory as executable)
	var scriptPath string
	searchPaths := []string{
		"./scripts/claude-hub.sh",
		"./claude-hub.sh",
	}

	// Add paths based on executable location
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		searchPaths = append(searchPaths,
			filepath.Join(exeDir, "scripts", "claude-hub.sh"),
			filepath.Join(exeDir, "claude-hub.sh"),
		)
	}
	log.Printf("executeHubCommand: search paths: %v", searchPaths)

	// Find first existing script
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			scriptPath = path
			log.Printf("executeHubCommand: found script at: %s", path)
			break
		}
	}

	// Verify script was found
	if scriptPath == "" {
		log.Printf("executeHubCommand: claude-hub.sh not found in any of: %v", searchPaths)
		return fmt.Errorf("claude-hub.sh not found in any of: %v", searchPaths)
	}

	// Get absolute path for reliability
	absScriptPath, err := filepath.Abs(scriptPath)
	if err != nil {
		absScriptPath = scriptPath
	}

	log.Printf("Executing: %s %s (via bash for complete process isolation)", absScriptPath, command)

	// Use bash -c with nohup and & to completely detach the process
	// This ensures the script survives even when the API process is killed
	// The script is run in its own session with stdin/stdout/stderr redirected
	bashCmd := fmt.Sprintf(
		"cd %s && nohup %s %s --production > /tmp/claude-hub-command-%s.log 2>&1 &",
		filepath.Dir(absScriptPath),
		absScriptPath,
		command,
		command,
	)

	cmd := exec.Command("bash", "-c", bashCmd)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Set process group to prevent signal propagation
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Run the command (this will return quickly since we use &)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	log.Printf("Command '%s' started successfully in background (log: /tmp/claude-hub-command-%s.log)", command, command)
	return nil
}

func executeSelectiveUpgrade(layer string, force bool, reload bool) (service.UpgradeResponse, error) {
	projectDir, err := filepath.Abs(".")
	if err != nil {
		return service.UpgradeResponse{Status: "error", Error: err.Error()}, err
	}

	req := service.UpgradeRequest{
		Layer:  layer,
		Force:  force,
		Reload: reload,
	}

	result, err := service.RunSelectiveUpgrade(projectDir, req)
	if err != nil {
		log.Printf("executeSelectiveUpgrade failed: layer=%s force=%t reload=%t err=%v", layer, force, reload, err)
		return result, err
	}

	return result, nil
}

// ============================================================
// Discussion Handlers (Agent Collaboration)
// ============================================================

// Discussion represents a collaboration thread
type Discussion struct {
	ID           string              `json:"id"`
	Title        string              `json:"title"`
	CreatedBy    string              `json:"created_by"`
	Participants []string            `json:"participants"`
	Status       string              `json:"status"` // active, closed
	RelatedTask  string              `json:"related_task,omitempty"`
	CreatedAt    string              `json:"created_at"`
	UpdatedAt    string              `json:"updated_at"`
	Messages     []DiscussionMessage `json:"messages,omitempty"`
}

// DiscussionMessage represents a message in a discussion
type DiscussionMessage struct {
	ID        int    `json:"id"`
	FromAgent string `json:"from_agent"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// In-memory storage for discussions (for simplicity)
var (
	discussions         = make(map[string]*Discussion)
	discussionsLock     sync.RWMutex
	discussionIDCounter int64
)

// listDiscussions returns all active discussions
func listDiscussions(w http.ResponseWriter, r *http.Request) {
	discussionsLock.RLock()
	defer discussionsLock.RUnlock()

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "active"
	}

	result := make([]*Discussion, 0)
	for _, d := range discussions {
		if status == "all" || d.Status == status {
			result = append(result, d)
		}
	}

	// Sort by updated_at descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt > result[j].UpdatedAt
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"discussions": result,
		"count":       len(result),
	})
}

// CreateDiscussionRequest is the request body for creating a discussion
type CreateDiscussionRequest struct {
	Title          string   `json:"title"`
	CreatedBy      string   `json:"created_by"`
	Participants   []string `json:"participants"`
	RelatedTask    string   `json:"related_task,omitempty"`
	InitialMessage string   `json:"initial_message,omitempty"`
}

// createDiscussion creates a new discussion thread
func createDiscussion(w http.ResponseWriter, r *http.Request) {
	var req CreateDiscussionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" || req.CreatedBy == "" {
		http.Error(w, "title and created_by are required", http.StatusBadRequest)
		return
	}

	// Ensure creator is in participants
	participantSet := make(map[string]bool)
	participantSet[req.CreatedBy] = true
	for _, p := range req.Participants {
		participantSet[p] = true
	}
	participants := make([]string, 0, len(participantSet))
	for p := range participantSet {
		participants = append(participants, p)
	}
	sort.Strings(participants)

	discussionsLock.Lock()
	discussionIDCounter++
	id := fmt.Sprintf("DISC-%d", discussionIDCounter)
	now := time.Now().Format(time.RFC3339)

	d := &Discussion{
		ID:           id,
		Title:        req.Title,
		CreatedBy:    req.CreatedBy,
		Participants: participants,
		Status:       "active",
		RelatedTask:  req.RelatedTask,
		CreatedAt:    now,
		UpdatedAt:    now,
		Messages:     make([]DiscussionMessage, 0),
	}

	// Add initial message if provided
	if req.InitialMessage != "" {
		d.Messages = append(d.Messages, DiscussionMessage{
			ID:        1,
			FromAgent: req.CreatedBy,
			Content:   req.InitialMessage,
			CreatedAt: now,
		})
	}

	discussions[id] = d
	discussionsLock.Unlock()

	// Notify participants via tmux
	for _, p := range participants {
		if p != req.CreatedBy {
			notifyMsg := fmt.Sprintf("[Discussion] %s があなたを議論「%s」に招待しました。ID: %s", req.CreatedBy, req.Title, id)
			pane, _ := getAgentPaneIndex(p)
			if pane >= 0 {
				_ = sendTmuxMessage(pane, notifyMsg)
			}
		}
	}

	log.Printf("Discussion created: %s by %s, participants: %v", id, req.CreatedBy, participants)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(d)
}

// getDiscussion returns a specific discussion with all messages
func getDiscussion(w http.ResponseWriter, r *http.Request) {
	discussionID := chi.URLParam(r, "discussionID")

	discussionsLock.RLock()
	d, exists := discussions[discussionID]
	discussionsLock.RUnlock()

	if !exists {
		http.Error(w, "Discussion not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(d)
}

// ReplyRequest is the request body for replying to a discussion
type ReplyRequest struct {
	FromAgent string `json:"from_agent"`
	Content   string `json:"content"`
}

// replyToDiscussion adds a message to a discussion
func replyToDiscussion(w http.ResponseWriter, r *http.Request) {
	discussionID := chi.URLParam(r, "discussionID")

	var req ReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FromAgent == "" || req.Content == "" {
		http.Error(w, "from_agent and content are required", http.StatusBadRequest)
		return
	}

	discussionsLock.Lock()
	d, exists := discussions[discussionID]
	if !exists {
		discussionsLock.Unlock()
		http.Error(w, "Discussion not found", http.StatusNotFound)
		return
	}

	if d.Status != "active" {
		discussionsLock.Unlock()
		http.Error(w, "Discussion is closed", http.StatusBadRequest)
		return
	}

	// Check if agent is a participant
	isParticipant := false
	for _, p := range d.Participants {
		if p == req.FromAgent {
			isParticipant = true
			break
		}
	}

	// Auto-add new participant if not already
	if !isParticipant {
		d.Participants = append(d.Participants, req.FromAgent)
		sort.Strings(d.Participants)
	}

	now := time.Now().Format(time.RFC3339)
	msg := DiscussionMessage{
		ID:        len(d.Messages) + 1,
		FromAgent: req.FromAgent,
		Content:   req.Content,
		CreatedAt: now,
	}
	d.Messages = append(d.Messages, msg)
	d.UpdatedAt = now
	discussionsLock.Unlock()

	// Notify other participants
	for _, p := range d.Participants {
		if p != req.FromAgent {
			notifyMsg := fmt.Sprintf("[%s] %s: %s", d.Title, req.FromAgent, truncateString(req.Content, 100))
			pane, _ := getAgentPaneIndex(p)
			if pane >= 0 {
				_ = sendTmuxMessage(pane, notifyMsg)
			}
		}
	}

	log.Printf("Discussion %s: reply from %s", discussionID, req.FromAgent)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": msg,
	})
}

// closeDiscussion closes a discussion
func closeDiscussion(w http.ResponseWriter, r *http.Request) {
	discussionID := chi.URLParam(r, "discussionID")

	discussionsLock.Lock()
	d, exists := discussions[discussionID]
	if !exists {
		discussionsLock.Unlock()
		http.Error(w, "Discussion not found", http.StatusNotFound)
		return
	}

	d.Status = "closed"
	d.UpdatedAt = time.Now().Format(time.RFC3339)
	discussionsLock.Unlock()

	log.Printf("Discussion %s closed", discussionID)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"discussion": d,
	})
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// --- App Management Handlers ---

// listAppsHandler returns all configured apps with their status
func listAppsHandler(w http.ResponseWriter, r *http.Request) {
	if appManager == nil {
		http.Error(w, "App manager not initialized", http.StatusServiceUnavailable)
		return
	}

	apps := appManager.ListAppsWithRefresh()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"apps":  apps,
		"count": len(apps),
	})
}

// refreshAllAppsHandler refreshes status of all apps
func refreshAllAppsHandler(w http.ResponseWriter, r *http.Request) {
	if appManager == nil {
		http.Error(w, "App manager not initialized", http.StatusServiceUnavailable)
		return
	}

	appManager.RefreshAllStatuses()
	apps := appManager.ListApps()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"apps":  apps,
		"count": len(apps),
	})
}

// getAppHandler returns the status of a specific app
func getAppHandler(w http.ResponseWriter, r *http.Request) {
	if appManager == nil {
		http.Error(w, "App manager not initialized", http.StatusServiceUnavailable)
		return
	}

	appID := chi.URLParam(r, "appID")
	app, err := appManager.GetApp(appID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(app)
}

// getAppStatusHandler returns refreshed status of a specific app
func getAppStatusHandler(w http.ResponseWriter, r *http.Request) {
	if appManager == nil {
		http.Error(w, "App manager not initialized", http.StatusServiceUnavailable)
		return
	}

	appID := chi.URLParam(r, "appID")
	status, err := appManager.RefreshStatus(appID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// buildAppHandler builds an application
func buildAppHandler(w http.ResponseWriter, r *http.Request) {
	if appManager == nil {
		http.Error(w, "App manager not initialized", http.StatusServiceUnavailable)
		return
	}

	appID := chi.URLParam(r, "appID")
	if err := appManager.BuildApp(appID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	app, _ := appManager.GetApp(appID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"app":     app,
	})
}

// startAppHandler starts an application
func startAppHandler(w http.ResponseWriter, r *http.Request) {
	if appManager == nil {
		http.Error(w, "App manager not initialized", http.StatusServiceUnavailable)
		return
	}

	appID := chi.URLParam(r, "appID")
	if err := appManager.StartApp(appID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	app, _ := appManager.GetApp(appID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"app":     app,
	})
}

// stopAppHandler stops an application
func stopAppHandler(w http.ResponseWriter, r *http.Request) {
	if appManager == nil {
		http.Error(w, "App manager not initialized", http.StatusServiceUnavailable)
		return
	}

	appID := chi.URLParam(r, "appID")
	if err := appManager.StopApp(appID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	app, _ := appManager.GetApp(appID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"app":     app,
	})
}

// restartAppHandler restarts an application
func restartAppHandler(w http.ResponseWriter, r *http.Request) {
	if appManager == nil {
		http.Error(w, "App manager not initialized", http.StatusServiceUnavailable)
		return
	}

	appID := chi.URLParam(r, "appID")
	if err := appManager.RestartApp(appID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	app, _ := appManager.GetApp(appID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"app":     app,
	})
}

// getAppLogsHandler returns recent logs for an application
func getAppLogsHandler(w http.ResponseWriter, r *http.Request) {
	if appManager == nil {
		http.Error(w, "App manager not initialized", http.StatusServiceUnavailable)
		return
	}

	appID := chi.URLParam(r, "appID")

	lines := 100
	if l := r.URL.Query().Get("lines"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			lines = n
		}
	}

	logs, err := appManager.GetLogs(appID, lines)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"app_id": appID,
		"lines":  lines,
		"logs":   logs,
	})
}

// dashboardProxyHandler proxies requests to an app's dashboard server
func dashboardProxyHandler(w http.ResponseWriter, r *http.Request) {
	if appManager == nil {
		http.Error(w, "App manager not initialized", http.StatusServiceUnavailable)
		return
	}

	appID := chi.URLParam(r, "appID")
	appCfg, err := appManager.GetAppConfig(appID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if appCfg.Port == 0 {
		http.Error(w, "App has no dashboard port configured", http.StatusBadRequest)
		return
	}

	targetURL, err := url.Parse(fmt.Sprintf("http://localhost:%d", appCfg.Port))
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Strip the API prefix from the path before proxying
	prefix := fmt.Sprintf("/api/v1/apps/%s/dashboard", appID)
	originalPath := r.URL.Path
	r.URL.Path = strings.TrimPrefix(originalPath, prefix)
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}

	r.Host = targetURL.Host
	proxy.ServeHTTP(w, r)
}

// getAppTradesHandler returns trade data from an app's data directory
func getAppTradesHandler(w http.ResponseWriter, r *http.Request) {
	if appManager == nil {
		http.Error(w, "App manager not initialized", http.StatusServiceUnavailable)
		return
	}

	appID := chi.URLParam(r, "appID")
	appStatus, err := appManager.GetApp(appID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	appPath := appStatus.Path
	if appPath == "" {
		http.Error(w, "App path not configured", http.StatusBadRequest)
		return
	}

	// Try SQLite database first (current format)
	dbPath := filepath.Join(appPath, "data", "trading.db")
	if _, err := os.Stat(dbPath); err == nil {
		result, err := readTradesFromDB(dbPath)
		if err != nil {
			log.Printf("Failed to read trades from DB %s: %v", dbPath, err)
			http.Error(w, fmt.Sprintf("Failed to read trades: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}

	// Fallback to JSON file (legacy format)
	tradesFile := filepath.Join(appPath, "data", "paper_trades.json")
	data, err := os.ReadFile(tradesFile)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"trades":     []interface{}{},
				"updated_at": "",
			})
			return
		}
		http.Error(w, fmt.Sprintf("Failed to read trades: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// readTradesFromDB reads trade data from a SQLite database
func readTradesFromDB(dbPath string) (map[string]interface{}, error) {
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT id, symbol, side, entry_price, exit_price, size, pnl,
		       entry_time, exit_time, exit_reason
		FROM trades
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades: %w", err)
	}
	defer rows.Close()

	var trades []map[string]interface{}
	var latestTime string

	for rows.Next() {
		var (
			id                   int
			symbol, side         string
			entryPrice           float64
			exitPrice            *float64
			size                 float64
			pnl                  *float64
			entryTime            string
			exitTime, exitReason *string
		)

		if err := rows.Scan(&id, &symbol, &side, &entryPrice, &exitPrice, &size, &pnl,
			&entryTime, &exitTime, &exitReason); err != nil {
			continue
		}

		trade := map[string]interface{}{
			"id":     id,
			"symbol": symbol,
			"side":   side,
			"size":   size,
		}

		if exitPrice != nil && pnl != nil {
			trade["action"] = "close"
			trade["price"] = *exitPrice
			trade["pnl"] = *pnl
			trade["pnl_pct"] = nil
			if exitTime != nil {
				trade["timestamp"] = *exitTime
				latestTime = *exitTime
			}
			if exitReason != nil {
				trade["exit_reason"] = *exitReason
			}
		} else {
			trade["action"] = "open"
			trade["price"] = entryPrice
			trade["pnl"] = nil
			trade["pnl_pct"] = nil
			trade["timestamp"] = entryTime
		}

		trades = append(trades, trade)
	}

	if trades == nil {
		trades = []map[string]interface{}{}
	}

	if latestTime == "" {
		latestTime = time.Now().Format(time.RFC3339)
	}

	return map[string]interface{}{
		"trades":     trades,
		"updated_at": latestTime,
	}, nil
}

// handleGetUsage runs ccusage CLI and returns JSON usage data
func handleGetUsage(w http.ResponseWriter, r *http.Request) {
	reportType := r.URL.Query().Get("type")
	if reportType == "" {
		reportType = "daily"
	}
	if reportType != "daily" && reportType != "weekly" && reportType != "monthly" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "type must be daily, weekly, or monthly"})
		return
	}

	since := r.URL.Query().Get("since")
	args := []string{"ccusage@latest", reportType, "--json", "--offline"}
	if since != "" {
		args = append(args, "--since", since)
	}

	ctx := r.Context()
	cmd := exec.CommandContext(ctx, "npx", args...)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("ccusage error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to run ccusage: " + err.Error()})
		return
	}

	// Parse and forward the JSON directly
	var data map[string]interface{}
	if err := json.Unmarshal(output, &data); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to parse ccusage output"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
