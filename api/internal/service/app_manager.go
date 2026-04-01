package service

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/zono819/claude-agent-hub/api/internal/config"
)

// NOTE: encoding/json is used by Docker Compose functions (currently commented out).
// Re-add the import when re-enabling Docker support.

// AppStatus represents the runtime status of an application
type AppStatus struct {
	AppID       string          `json:"app_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Repo        string          `json:"repo"`
	Language    string          `json:"language,omitempty"`
	Type        string          `json:"type"`
	Port        int             `json:"port"`
	Status      string          `json:"status"` // running, stopped, starting, stopping, error, building, unknown
	Health      string          `json:"health"` // healthy, unhealthy, unknown
	PID         int             `json:"pid,omitempty"`
	Services    []ServiceStatus `json:"services,omitempty"`
	Path        string          `json:"path,omitempty"`
	LogFile     string          `json:"log_file,omitempty"`
	Error       string          `json:"error,omitempty"`
	UsesClaude  bool            `json:"uses_claude,omitempty"`
	Model       string          `json:"model,omitempty"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ServiceStatus represents the status of a docker compose service
type ServiceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Health string `json:"health,omitempty"`
}

// AppManager manages application lifecycle
type AppManager struct {
	config      *config.AppsConfig
	statuses    map[string]*AppStatus
	dataDir     string // base directory for pids/logs
	mu          sync.RWMutex
	lastRefresh time.Time
}

// NewAppManager creates a new AppManager
func NewAppManager(cfg *config.AppsConfig) *AppManager {
	dataDir := os.Getenv("APP_DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	m := &AppManager{
		config:   cfg,
		statuses: make(map[string]*AppStatus),
		dataDir:  dataDir,
	}

	// Ensure pid and log directories exist
	os.MkdirAll(filepath.Join(dataDir, "pids"), 0755)
	os.MkdirAll(filepath.Join(dataDir, "logs"), 0755)

	// Initialize statuses
	for id, app := range cfg.Apps {
		m.statuses[id] = &AppStatus{
			AppID:       id,
			Name:        app.Name,
			Description: app.Description,
			Repo:        app.Repo,
			Language:    app.Language,
			Type:        app.Type,
			Port:        app.Port,
			Status:      "unknown",
			Health:      "unknown",
			Path:        cfg.GetAppPath(id),
			LogFile:     m.getLogFilePath(id, app),
			UsesClaude:  app.UsesClaude,
			Model:       app.Model,
			UpdatedAt:   time.Now(),
		}
	}

	// Ensure log files exist for all apps
	for id, app := range cfg.Apps {
		logPath := m.getLogFilePath(id, app)
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			os.MkdirAll(filepath.Dir(logPath), 0755)
			os.WriteFile(logPath, []byte{}, 0644)
		}
	}

	return m
}

// ListApps returns all configured apps with their current status
func (m *AppManager) ListApps() []AppStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]AppStatus, 0, len(m.statuses))
	for _, s := range m.statuses {
		apps = append(apps, *s)
	}
	return apps
}

// ListAppsWithRefresh returns all apps with auto-refresh if stale (>10s)
func (m *AppManager) ListAppsWithRefresh() []AppStatus {
	m.mu.RLock()
	needsRefresh := time.Since(m.lastRefresh) > 10*time.Second
	m.mu.RUnlock()

	if needsRefresh {
		m.RefreshAllStatuses()
		m.mu.Lock()
		m.lastRefresh = time.Now()
		m.mu.Unlock()
	}

	return m.ListApps()
}

// GetApp returns the status of a specific app
func (m *AppManager) GetApp(appID string) (*AppStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, exists := m.statuses[appID]
	if !exists {
		return nil, fmt.Errorf("app %q not found", appID)
	}
	return s, nil
}

// GetAppConfig returns the configuration of a specific app
func (m *AppManager) GetAppConfig(appID string) (*config.AppConfig, error) {
	appCfg, exists := m.config.Apps[appID]
	if !exists {
		return nil, fmt.Errorf("app %q not found", appID)
	}
	return &appCfg, nil
}

// RefreshStatus refreshes the status of a specific app
func (m *AppManager) RefreshStatus(appID string) (*AppStatus, error) {
	appCfg, exists := m.config.Apps[appID]
	if !exists {
		return nil, fmt.Errorf("app %q not found", appID)
	}

	appPath := m.config.GetAppPath(appID)
	status := m.getAppStatus(appID, appCfg, appPath)

	m.mu.Lock()
	m.statuses[appID] = status
	m.mu.Unlock()

	return status, nil
}

// RefreshAllStatuses refreshes statuses of all apps
func (m *AppManager) RefreshAllStatuses() {
	var wg sync.WaitGroup
	for id, app := range m.config.Apps {
		wg.Add(1)
		go func(id string, app config.AppConfig) {
			defer wg.Done()
			appPath := m.config.GetAppPath(id)
			status := m.getAppStatus(id, app, appPath)
			m.mu.Lock()
			m.statuses[id] = status
			m.mu.Unlock()
		}(id, app)
	}
	wg.Wait()
}

// BuildApp builds an application
func (m *AppManager) BuildApp(appID string) error {
	appCfg, exists := m.config.Apps[appID]
	if !exists {
		return fmt.Errorf("app %q not found", appID)
	}

	if appCfg.BuildCommand == "" {
		return fmt.Errorf("no build_command configured for app %q", appID)
	}

	appPath := m.config.GetAppPath(appID)
	if appPath == "" {
		return fmt.Errorf("app %q has no configured path", appID)
	}

	m.mu.Lock()
	if s, ok := m.statuses[appID]; ok {
		s.Status = "building"
		s.Error = ""
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	workDir := m.resolveWorkDir(appPath, appCfg)
	if err := m.execCommand(workDir, "sh", "-c", appCfg.BuildCommand); err != nil {
		m.mu.Lock()
		if s, ok := m.statuses[appID]; ok {
			s.Status = "error"
			s.Error = fmt.Sprintf("build failed: %v", err)
			s.UpdatedAt = time.Now()
		}
		m.mu.Unlock()
		return fmt.Errorf("build failed: %w", err)
	}

	m.mu.Lock()
	if s, ok := m.statuses[appID]; ok {
		s.Status = "stopped"
		s.Error = ""
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	return nil
}

// StartApp starts an application
func (m *AppManager) StartApp(appID string) error {
	appCfg, exists := m.config.Apps[appID]
	if !exists {
		return fmt.Errorf("app %q not found", appID)
	}

	appPath := m.config.GetAppPath(appID)
	if appPath == "" {
		return fmt.Errorf("app %q has no configured path", appID)
	}

	// Check if already running
	if pid := m.readPidFile(appID, appCfg); pid > 0 && m.isProcessRunning(pid) {
		// Main process is running; start dashboard if needed
		if appCfg.DashboardCommand != "" {
			dashPid := m.readDashboardPidFile(appID)
			if dashPid <= 0 || !m.isProcessRunning(dashPid) {
				if err := m.startDashboardProcess(appID, appPath, appCfg); err != nil {
					log.Printf("Warning: failed to start dashboard for %s: %v", appID, err)
				}
				time.Sleep(1 * time.Second)
				m.RefreshStatus(appID)
				return nil
			}
		}
		return fmt.Errorf("app %q is already running (PID %d)", appID, pid)
	}

	m.mu.Lock()
	if s, ok := m.statuses[appID]; ok {
		s.Status = "starting"
		s.Error = ""
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	var err error
	switch appCfg.Type {
	case "self":
		// Self-managed: claude-agent-hub itself
		err = fmt.Errorf("app %q is self-managed and cannot be started via API", appID)
	case "docker-compose":
		// TODO: Docker Compose support disabled for memory conservation
		// err = m.startDockerCompose(appPath, appCfg)
		err = fmt.Errorf("docker-compose support disabled (use type: process instead)")
	case "docker":
		// TODO: Docker support disabled for memory conservation
		// err = m.startDocker(appID, appPath, appCfg)
		err = fmt.Errorf("docker support disabled (use type: process instead)")
	default:
		err = m.startProcess(appID, appPath, appCfg)
	}

	if err != nil {
		m.mu.Lock()
		if s, ok := m.statuses[appID]; ok {
			s.Status = "error"
			s.Error = err.Error()
			s.UpdatedAt = time.Now()
		}
		m.mu.Unlock()
		return err
	}

	// Wait briefly then refresh status
	time.Sleep(2 * time.Second)
	m.RefreshStatus(appID)
	return nil
}

// StopApp stops an application
func (m *AppManager) StopApp(appID string) error {
	appCfg, exists := m.config.Apps[appID]
	if !exists {
		return fmt.Errorf("app %q not found", appID)
	}

	appPath := m.config.GetAppPath(appID)
	if appPath == "" {
		return fmt.Errorf("app %q has no configured path", appID)
	}

	m.mu.Lock()
	if s, ok := m.statuses[appID]; ok {
		s.Status = "stopping"
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	var err error
	switch appCfg.Type {
	case "self":
		err = fmt.Errorf("app %q is self-managed and cannot be stopped via API", appID)
	case "docker-compose":
		// TODO: Docker Compose support disabled for memory conservation
		// err = m.stopDockerCompose(appPath, appCfg)
		err = fmt.Errorf("docker-compose support disabled")
	case "docker":
		// TODO: Docker support disabled for memory conservation
		// err = m.stopDocker(appID)
		err = fmt.Errorf("docker support disabled")
	default:
		err = m.stopProcess(appID, appCfg)
	}

	if err != nil {
		m.mu.Lock()
		if s, ok := m.statuses[appID]; ok {
			s.Status = "error"
			s.Error = err.Error()
			s.UpdatedAt = time.Now()
		}
		m.mu.Unlock()
		return err
	}

	m.mu.Lock()
	if s, ok := m.statuses[appID]; ok {
		s.Status = "stopped"
		s.Health = "unknown"
		s.PID = 0
		s.Services = nil
		s.Error = ""
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
	return nil
}

// RestartApp restarts an application
func (m *AppManager) RestartApp(appID string) error {
	if err := m.StopApp(appID); err != nil {
		log.Printf("Warning: error stopping app %s during restart: %v", appID, err)
	}
	time.Sleep(1 * time.Second)
	return m.StartApp(appID)
}

// GetLogs returns recent logs for an application
func (m *AppManager) GetLogs(appID string, lines int) (string, error) {
	appCfg, exists := m.config.Apps[appID]
	if !exists {
		return "", fmt.Errorf("app %q not found", appID)
	}

	if lines <= 0 {
		lines = 100
	}
	if lines > 1000 {
		lines = 1000
	}

	switch appCfg.Type {
	case "docker-compose":
		// TODO: Docker Compose logs disabled for memory conservation
		return "", fmt.Errorf("docker-compose logs disabled (use type: process)")
	case "docker":
		// TODO: Docker logs disabled for memory conservation
		return "", fmt.Errorf("docker logs disabled (use type: process)")
	default:
		return m.getProcessLogs(appID, appCfg, lines)
	}
}

// --- Process operations (main execution path) ---

func (m *AppManager) startProcess(appID, appPath string, cfg config.AppConfig) error {
	if cfg.StartCommand == "" {
		return fmt.Errorf("no start_command configured for app %q", appID)
	}

	workDir := m.resolveWorkDir(appPath, cfg)

	// Open log file for output
	logPath := m.getLogFilePath(appID, cfg)
	os.MkdirAll(filepath.Dir(logPath), 0755)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}

	// Write startup marker
	fmt.Fprintf(logFile, "\n=== %s started at %s ===\n", appID, time.Now().Format(time.RFC3339))

	// Parse and execute command
	cmd := exec.Command("sh", "-c", cfg.StartCommand)
	cmd.Dir = workDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Load environment from env file if specified
	cmd.Env = os.Environ()
	if cfg.EnvFile != "" {
		envPath := cfg.EnvFile
		if !filepath.IsAbs(envPath) {
			envPath = filepath.Join(appPath, envPath)
		}
		if envVars, err := loadEnvFile(envPath); err == nil {
			cmd.Env = append(cmd.Env, envVars...)
		}
	}

	// Set process group so we can kill the whole group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	pid := cmd.Process.Pid

	// Write PID file
	pidPath := m.getPidFilePath(appID, cfg)
	os.MkdirAll(filepath.Dir(pidPath), 0755)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		log.Printf("Warning: failed to write PID file for %s: %v", appID, err)
	}

	log.Printf("Started app %s (PID %d), logs: %s", appID, pid, logPath)

	// Background goroutine to wait for process exit and clean up
	go func() {
		err := cmd.Wait()
		logFile.Close()

		// Clean up PID file
		os.Remove(pidPath)

		m.mu.Lock()
		if s, ok := m.statuses[appID]; ok {
			if err != nil {
				s.Status = "error"
				s.Error = fmt.Sprintf("process exited: %v", err)
			} else {
				s.Status = "stopped"
				s.Error = ""
			}
			s.PID = 0
			s.Health = "unknown"
			s.UpdatedAt = time.Now()
		}
		m.mu.Unlock()

		log.Printf("App %s (PID %d) exited: %v", appID, pid, err)
	}()

	// Start dashboard server if configured
	if cfg.DashboardCommand != "" {
		if err := m.startDashboardProcess(appID, appPath, cfg); err != nil {
			log.Printf("Warning: failed to start dashboard for %s: %v", appID, err)
		}
	}

	return nil
}

// startDashboardProcess starts the dashboard HTTP server as a separate process
func (m *AppManager) startDashboardProcess(appID, appPath string, cfg config.AppConfig) error {
	// Check if already running
	dashPid := m.readDashboardPidFile(appID)
	if dashPid > 0 && m.isProcessRunning(dashPid) {
		log.Printf("Dashboard for %s already running (PID %d)", appID, dashPid)
		return nil
	}

	workDir := m.resolveWorkDir(appPath, cfg)

	// Open log file for dashboard output
	logPath := m.getLogFilePath(appID, cfg)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	fmt.Fprintf(logFile, "\n=== %s dashboard started at %s ===\n", appID, time.Now().Format(time.RFC3339))

	cmd := exec.Command("sh", "-c", cfg.DashboardCommand)
	cmd.Dir = workDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start dashboard process: %w", err)
	}

	dashPid = cmd.Process.Pid

	// Write dashboard PID file
	pidPath := m.getDashboardPidFilePath(appID)
	os.MkdirAll(filepath.Dir(pidPath), 0755)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(dashPid)), 0644); err != nil {
		log.Printf("Warning: failed to write dashboard PID file for %s: %v", appID, err)
	}

	log.Printf("Started dashboard for %s (PID %d)", appID, dashPid)

	// Background goroutine to clean up on exit
	go func() {
		cmd.Wait()
		logFile.Close()
		os.Remove(pidPath)
		log.Printf("Dashboard for %s (PID %d) exited", appID, dashPid)
	}()

	return nil
}

func (m *AppManager) stopProcess(appID string, cfg config.AppConfig) error {
	// Stop dashboard process first if running
	m.stopDashboardProcess(appID)

	pid := m.readPidFile(appID, cfg)
	if pid <= 0 {
		// No PID file, try pkill as fallback
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		exec.CommandContext(ctx, "pkill", "-f", cfg.StartCommand).Run()
		return nil
	}

	// Check if process is running
	if !m.isProcessRunning(pid) {
		m.removePidFile(appID, cfg)
		return nil
	}

	// Graceful shutdown: SIGTERM → wait → SIGKILL
	log.Printf("Stopping app %s (PID %d) with SIGTERM", appID, pid)

	// Send SIGTERM to process group
	pgid, err := syscall.Getpgid(pid)
	if err == nil {
		syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		syscall.Kill(pid, syscall.SIGTERM)
	}

	// Wait up to 10 seconds for graceful shutdown
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if !m.isProcessRunning(pid) {
			m.removePidFile(appID, cfg)
			log.Printf("App %s (PID %d) stopped gracefully", appID, pid)
			return nil
		}
	}

	// Force kill
	log.Printf("App %s (PID %d) did not stop gracefully, sending SIGKILL", appID, pid)
	if pgid, err := syscall.Getpgid(pid); err == nil {
		syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		syscall.Kill(pid, syscall.SIGKILL)
	}

	time.Sleep(500 * time.Millisecond)
	m.removePidFile(appID, cfg)
	return nil
}

func (m *AppManager) getProcessLogs(appID string, cfg config.AppConfig, lines int) (string, error) {
	logPath := m.getLogFilePath(appID, cfg)

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("No log file found for %s (%s)", appID, logPath), nil
		}
		return "", fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Read last N lines using a ring buffer approach
	scanner := bufio.NewScanner(file)
	var allLines []string
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if len(allLines) <= lines {
		return strings.Join(allLines, "\n"), nil
	}
	return strings.Join(allLines[len(allLines)-lines:], "\n"), nil
}

// --- PID file management ---

func (m *AppManager) getPidFilePath(appID string, cfg config.AppConfig) string {
	if cfg.PidFile != "" {
		return cfg.PidFile
	}
	return filepath.Join(m.dataDir, "pids", appID+".pid")
}

func (m *AppManager) getLogFilePath(appID string, cfg config.AppConfig) string {
	if cfg.LogFile != "" {
		return cfg.LogFile
	}
	return filepath.Join(m.dataDir, "logs", appID+".log")
}

func (m *AppManager) readPidFile(appID string, cfg config.AppConfig) int {
	pidPath := m.getPidFilePath(appID, cfg)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

func (m *AppManager) removePidFile(appID string, cfg config.AppConfig) {
	pidPath := m.getPidFilePath(appID, cfg)
	os.Remove(pidPath)
}

// --- Dashboard PID file management ---

func (m *AppManager) getDashboardPidFilePath(appID string) string {
	return filepath.Join(m.dataDir, "pids", appID+"-dashboard.pid")
}

func (m *AppManager) readDashboardPidFile(appID string) int {
	pidPath := m.getDashboardPidFilePath(appID)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

func (m *AppManager) stopDashboardProcess(appID string) {
	dashPid := m.readDashboardPidFile(appID)
	if dashPid <= 0 || !m.isProcessRunning(dashPid) {
		os.Remove(m.getDashboardPidFilePath(appID))
		return
	}

	log.Printf("Stopping dashboard for %s (PID %d)", appID, dashPid)
	if pgid, err := syscall.Getpgid(dashPid); err == nil {
		syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		syscall.Kill(dashPid, syscall.SIGTERM)
	}

	// Wait briefly for graceful shutdown
	for i := 0; i < 6; i++ {
		time.Sleep(500 * time.Millisecond)
		if !m.isProcessRunning(dashPid) {
			break
		}
	}

	// Force kill if still running
	if m.isProcessRunning(dashPid) {
		if pgid, err := syscall.Getpgid(dashPid); err == nil {
			syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			syscall.Kill(dashPid, syscall.SIGKILL)
		}
	}

	os.Remove(m.getDashboardPidFilePath(appID))
	log.Printf("Stopped dashboard for %s", appID)
}

func (m *AppManager) isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	// kill -0 checks if process exists without sending a signal
	err := syscall.Kill(pid, 0)
	return err == nil
}

// --- Health check ---

func (m *AppManager) checkHealth(cfg config.AppConfig) string {
	if cfg.HealthCheck.Type == "http" && cfg.HealthCheck.Port > 0 {
		url := fmt.Sprintf("http://localhost:%d%s", cfg.HealthCheck.Port, cfg.HealthCheck.Path)
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return "unhealthy"
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return "healthy"
		}
		return "unhealthy"
	}
	return "unknown"
}

// --- Helpers ---

func (m *AppManager) resolveWorkDir(appPath string, cfg config.AppConfig) string {
	if cfg.WorkDir != "" {
		if filepath.IsAbs(cfg.WorkDir) {
			return cfg.WorkDir
		}
		return filepath.Join(appPath, cfg.WorkDir)
	}
	return appPath
}

func (m *AppManager) getAppStatus(appID string, cfg config.AppConfig, appPath string) *AppStatus {
	status := &AppStatus{
		AppID:       appID,
		Name:        cfg.Name,
		Description: cfg.Description,
		Repo:        cfg.Repo,
		Language:    cfg.Language,
		Type:        cfg.Type,
		Port:        cfg.Port,
		Status:      "stopped",
		Health:      "unknown",
		Path:        appPath,
		LogFile:     m.getLogFilePath(appID, cfg),
		UsesClaude:  cfg.UsesClaude,
		Model:       cfg.Model,
		UpdatedAt:   time.Now(),
	}

	// Self-managed apps don't need a path
	if cfg.Type == "self" {
		status.Status = "running"
		if cfg.HealthCheck.Type == "http" {
			status.Health = m.checkHealth(cfg)
		} else {
			status.Health = "healthy"
		}
		return status
	}

	if appPath == "" {
		// pathが未設定でもhealthcheckでステータス検出を試みる
		// （外部起動されたプロセスやstart_command不要のアプリ向け）
		if cfg.HealthCheck.Type == "http" {
			health := m.checkHealth(cfg)
			status.Health = health
			if health == "healthy" {
				status.Status = "running"
				return status
			}
		}
		status.Status = "not_configured"
		return status
	}

	switch cfg.Type {
	case "docker-compose":
		// TODO: Docker Compose status check disabled for memory conservation
		// Re-enable by uncommenting the block below:
		// services, err := m.getDockerComposeStatus(appPath, cfg)
		// if err == nil { ... }
		status.Status = "stopped"
		status.Error = "docker-compose disabled (use type: process)"
	case "docker":
		// TODO: Docker status check disabled for memory conservation
		// containerStatus, health, _ := m.getDockerStatus(appID)
		status.Status = "stopped"
		status.Error = "docker disabled (use type: process)"
	default:
		// Process-based: check PID file + process existence
		pid := m.readPidFile(appID, cfg)
		if pid > 0 && m.isProcessRunning(pid) {
			status.Status = "running"
			status.PID = pid
		}

		// HTTP health check if available
		if cfg.HealthCheck.Type == "http" {
			health := m.checkHealth(cfg)
			status.Health = health
			// If health check is configured but no PID, use health to determine status
			if pid <= 0 && health == "healthy" {
				status.Status = "running"
			}
		}
	}

	// Final health check for running apps
	if status.Status == "running" && cfg.HealthCheck.Type == "http" {
		status.Health = m.checkHealth(cfg)
	}

	return status
}

func (m *AppManager) execCommand(dir string, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w\nOutput: %s", name, strings.Join(args, " "), err, string(output))
	}
	return nil
}

// loadEnvFile reads a .env file and returns key=value pairs
func loadEnvFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var envVars []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "=") {
			envVars = append(envVars, line)
		}
	}
	return envVars, scanner.Err()
}

// --- Docker Compose operations (TODO: disabled for memory conservation) ---
// Uncomment these functions to re-enable Docker Compose support.

/*
func (m *AppManager) startDockerCompose(appPath string, cfg config.AppConfig) error {
	composeFile := cfg.ComposeFile
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}
	args := []string{"compose", "-f", composeFile, "up", "-d"}
	return m.execCommand(appPath, "docker", args...)
}

func (m *AppManager) stopDockerCompose(appPath string, cfg config.AppConfig) error {
	composeFile := cfg.ComposeFile
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}
	args := []string{"compose", "-f", composeFile, "down"}
	return m.execCommand(appPath, "docker", args...)
}

func (m *AppManager) getDockerComposeStatus(appPath string, cfg config.AppConfig) ([]ServiceStatus, error) {
	composeFile := cfg.ComposeFile
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "ps", "--format", "json")
	cmd.Dir = appPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker compose ps failed: %w", err)
	}

	var services []ServiceStatus
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		var container struct {
			Service string `json:"Service"`
			State   string `json:"State"`
			Health  string `json:"Health"`
		}
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			continue
		}
		services = append(services, ServiceStatus{
			Name:   container.Service,
			Status: container.State,
			Health: container.Health,
		})
	}
	return services, nil
}

func (m *AppManager) getDockerComposeLogs(appPath string, cfg config.AppConfig, lines int) (string, error) {
	composeFile := cfg.ComposeFile
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "logs", "--tail", fmt.Sprintf("%d", lines), "--no-color")
	cmd.Dir = appPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("docker compose logs failed: %w", err)
	}
	return string(output), nil
}
*/

// --- Docker (standalone) operations (TODO: disabled for memory conservation) ---
// Uncomment these functions to re-enable Docker support.

/*
func (m *AppManager) startDocker(appID, appPath string, cfg config.AppConfig) error {
	dockerfile := cfg.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	if err := m.execCommand(appPath, "docker", "build", "-t", appID, "-f", dockerfile, "."); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}
	args := []string{"run", "-d", "--name", appID, "--restart", "unless-stopped"}
	if cfg.Port > 0 {
		args = append(args, "-p", fmt.Sprintf("%d:%d", cfg.Port, cfg.Port))
	}
	args = append(args, appID)
	return m.execCommand(appPath, "docker", args...)
}

func (m *AppManager) stopDocker(appID string) error {
	m.execCommand("", "docker", "stop", appID)
	return m.execCommand("", "docker", "rm", "-f", appID)
}

func (m *AppManager) getDockerStatus(appID string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Status}}|{{.State.Health.Status}}", appID)
	output, err := cmd.Output()
	if err != nil {
		return "stopped", "unknown", nil
	}
	parts := strings.SplitN(strings.TrimSpace(string(output)), "|", 2)
	status := parts[0]
	health := "unknown"
	if len(parts) > 1 && parts[1] != "" {
		health = parts[1]
	}
	return status, health, nil
}

func (m *AppManager) getDockerLogs(appID string, lines int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", fmt.Sprintf("%d", lines), appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("docker logs failed: %w", err)
	}
	return string(output), nil
}
*/
