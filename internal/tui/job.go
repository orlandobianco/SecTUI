package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// backgroundActions lists action IDs that run as external processes.
var backgroundActions = map[string]bool{
	"clam_scan_home": true,
	"clam_scan_tmp":  true,
	"clam_update_db": true,
	"cs_hub_update":  true,
}

// --- Messages ---

type jobTickMsg struct{}

// --- Job ---

// Job represents a persistent background action tracked via files on disk.
type Job struct {
	ID        string    `json:"id"`
	ToolID    string    `json:"tool_id"`
	ActionID  string    `json:"action_id"`
	Label     string    `json:"label"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Done      bool      `json:"done"`
	ExitCode  int       `json:"exit_code"`
	Success   bool      `json:"success"`
	Message   string    `json:"message"`

	// Runtime only (not serialized)
	logPath  string
	metaPath string
	lastSize int64
}

func (j *Job) LogPath() string {
	if j.logPath != "" {
		return j.logPath
	}
	dir, _ := jobsDir()
	safeID := strings.ReplaceAll(j.ID, ":", "_")
	return filepath.Join(dir, safeID+".log")
}

func (j *Job) MetaPath() string {
	if j.metaPath != "" {
		return j.metaPath
	}
	dir, _ := jobsDir()
	safeID := strings.ReplaceAll(j.ID, ":", "_")
	return filepath.Join(dir, safeID+".json")
}

// ReadNewOutput reads new content from the log file since last read.
func (j *Job) ReadNewOutput() string {
	f, err := os.Open(j.LogPath())
	if err != nil {
		return ""
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return ""
	}

	size := info.Size()
	if size <= j.lastSize {
		return ""
	}

	if _, err := f.Seek(j.lastSize, io.SeekStart); err != nil {
		return ""
	}

	buf := make([]byte, size-j.lastSize)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return ""
	}

	j.lastSize = j.lastSize + int64(n)
	return string(buf[:n])
}

// FullOutput reads the entire log file content.
func (j *Job) FullOutput() string {
	data, err := os.ReadFile(j.LogPath())
	if err != nil {
		return ""
	}
	return string(data)
}

// IsAlive checks if the job's process is still running.
func (j *Job) IsAlive() bool {
	if j.PID <= 0 {
		return false
	}
	proc, err := os.FindProcess(j.PID)
	if err != nil {
		return false
	}
	// On Unix, signal 0 checks if process exists without actually sending a signal.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// Elapsed returns the duration since the job started.
func (j *Job) Elapsed() time.Duration {
	return time.Since(j.StartedAt).Truncate(time.Second)
}

// ReloadMeta re-reads the .json meta file to pick up changes written by the external process.
func (j *Job) ReloadMeta() error {
	data, err := os.ReadFile(j.MetaPath())
	if err != nil {
		return err
	}
	return json.Unmarshal(data, j)
}

// SaveMeta writes the job state to disk.
func (j *Job) SaveMeta() error {
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(j.MetaPath(), data, 0o644)
}

// --- JobManager ---

type JobManager struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func NewJobManager() *JobManager {
	return &JobManager{jobs: make(map[string]*Job)}
}

// LaunchJob starts a detached `sectui job exec tool:action` process.
func (jm *JobManager) LaunchJob(toolID, actionID, label string) (*Job, error) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	id := fmt.Sprintf("%s:%s", toolID, actionID)

	dir, err := jobsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	job := &Job{
		ID:        id,
		ToolID:    toolID,
		ActionID:  actionID,
		Label:     label,
		StartedAt: time.Now(),
	}
	job.logPath = job.LogPath()
	job.metaPath = job.MetaPath()

	// Find the sectui binary path.
	binary, err := os.Executable()
	if err != nil {
		binary = os.Args[0]
	}

	// Create log file.
	logFile, err := os.Create(job.logPath)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}

	// Build the detached command.
	cmd := exec.Command(binary, "job", "exec", id)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	// Detach from parent process so it survives SecTUI exit.
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start job process: %w", err)
	}

	// Close log file in parent — the child owns it now.
	logFile.Close()

	job.PID = cmd.Process.Pid

	// Save meta before releasing the process.
	if err := job.SaveMeta(); err != nil {
		return nil, fmt.Errorf("save job meta: %w", err)
	}

	// Release the process so it's not waited on by the parent.
	cmd.Process.Release()

	jm.jobs[id] = job
	return job, nil
}

// LoadFromDisk reads all job .json files and rebuilds state.
func (jm *JobManager) LoadFromDisk() error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	dir, err := jobsDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var job Job
		if err := json.Unmarshal(data, &job); err != nil {
			continue
		}

		job.logPath = job.LogPath()
		job.metaPath = job.MetaPath()

		// If job wasn't marked done but process is dead, reload and check.
		if !job.Done {
			// Try reloading in case the process updated the file.
			_ = job.ReloadMeta()
			if !job.Done && !job.IsAlive() {
				// Process died without updating meta — mark as failed.
				job.Done = true
				job.Success = false
				job.Message = "Process terminated unexpectedly"
				_ = job.SaveMeta()
			}
		}

		jm.jobs[job.ID] = &job
	}

	// Clean old completed jobs.
	jm.cleanOldLocked(24 * time.Hour)

	return nil
}

// cleanOldLocked removes completed jobs older than maxAge. Must hold lock.
func (jm *JobManager) cleanOldLocked(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	for id, job := range jm.jobs {
		if job.Done && job.StartedAt.Before(cutoff) {
			os.Remove(job.LogPath())
			os.Remove(job.MetaPath())
			delete(jm.jobs, id)
		}
	}
}

func (jm *JobManager) Get(jobID string) *Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	return jm.jobs[jobID]
}

func (jm *JobManager) HasRunning(toolID string) bool {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	for _, j := range jm.jobs {
		if j.ToolID == toolID && !j.Done {
			return true
		}
	}
	return false
}

func (jm *JobManager) HasAnyRunning() bool {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	for _, j := range jm.jobs {
		if !j.Done {
			return true
		}
	}
	return false
}

func (jm *JobManager) RunningJobs() []*Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	var running []*Job
	for _, j := range jm.jobs {
		if !j.Done {
			running = append(running, j)
		}
	}
	return running
}

func (jm *JobManager) RunningJobFor(toolID string) *Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	for _, j := range jm.jobs {
		if j.ToolID == toolID && !j.Done {
			return j
		}
	}
	return nil
}

func (jm *JobManager) CompletedJobFor(toolID string) *Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	for _, j := range jm.jobs {
		if j.ToolID == toolID && j.Done {
			return j
		}
	}
	return nil
}

func (jm *JobManager) Dismiss(jobID string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if job, ok := jm.jobs[jobID]; ok && job.Done {
		os.Remove(job.LogPath())
		os.Remove(job.MetaPath())
		delete(jm.jobs, jobID)
	}
}

// CheckRunningJobs polls running jobs for completion.
// Returns true if any job status changed.
func (jm *JobManager) CheckRunningJobs() bool {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	changed := false
	for _, job := range jm.jobs {
		if job.Done {
			continue
		}
		// Try reloading meta — the external process updates it on completion.
		_ = job.ReloadMeta()
		if job.Done {
			changed = true
			continue
		}
		// If process is no longer alive, mark done.
		if !job.IsAlive() {
			job.Done = true
			job.Success = false
			job.Message = "Process terminated unexpectedly"
			_ = job.SaveMeta()
			changed = true
		}
	}
	return changed
}

// FormatElapsed formats a duration into a human-readable string.
func FormatElapsed(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m < 60 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := m / 60
	m = m % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

// jobsDir returns the path to ~/.config/sectui/jobs/
func jobsDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sectui", "jobs"), nil
}

// FindToolManager looks up a ToolManager by toolID from a list of SecurityTools.
// Exported so the job exec subcommand can use it.
func FindToolManagerFromTools(tools []interface{ ID() string }, id string) interface{} {
	for _, t := range tools {
		if t.ID() == id {
			return t
		}
	}
	return nil
}
