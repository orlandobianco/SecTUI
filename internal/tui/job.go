package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/orlandobianco/SecTUI/internal/core"
)

// spinnerFrames are braille characters for the sidebar/header animation.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// backgroundActions lists action IDs that should run in background (long-running).
// Everything else runs synchronously (fast queries/status checks).
var backgroundActions = map[string]bool{
	"clam_scan_home": true,
	"clam_scan_tmp":  true,
	"clam_update_db": true,
	"cs_hub_update":  true,
}

// --- Messages ---

type startJobMsg struct {
	Job *Job
}

type jobCompletedMsg struct {
	JobID  string
	Result core.ActionResult
}

type jobTickMsg struct{}

// --- Job ---

// Job represents a background tool action.
type Job struct {
	ID        string
	ToolID    string
	ActionID  string
	Label     string
	StartedAt time.Time
	Done      bool
	Result    *core.ActionResult

	mu     sync.Mutex
	output strings.Builder
}

// AppendOutput appends text to the job's output buffer (thread-safe).
func (j *Job) AppendOutput(s string) {
	j.mu.Lock()
	j.output.WriteString(s)
	j.mu.Unlock()
}

// Output returns the current output buffer (thread-safe).
func (j *Job) Output() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.output.String()
}

// Elapsed returns the duration since the job started.
func (j *Job) Elapsed() time.Duration {
	return time.Since(j.StartedAt).Truncate(time.Second)
}

// --- JobManager ---

// JobManager tracks all active and recently completed background jobs.
type JobManager struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func NewJobManager() *JobManager {
	return &JobManager{jobs: make(map[string]*Job)}
}

// Start creates and registers a new running job.
func (jm *JobManager) Start(toolID, actionID, label string) *Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	id := fmt.Sprintf("%s:%s", toolID, actionID)
	job := &Job{
		ID:        id,
		ToolID:    toolID,
		ActionID:  actionID,
		Label:     label,
		StartedAt: time.Now(),
	}
	jm.jobs[id] = job
	return job
}

// Complete marks a job as done and stores its result.
func (jm *JobManager) Complete(jobID string, result core.ActionResult) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if job, ok := jm.jobs[jobID]; ok {
		job.Done = true
		job.Result = &result
	}
}

// Get returns a job by ID, or nil.
func (jm *JobManager) Get(jobID string) *Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	return jm.jobs[jobID]
}

// HasRunning returns true if the given tool has any active (not done) jobs.
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

// HasAnyRunning returns true if any job is still running.
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

// RunningJobs returns all currently active jobs.
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

// CompletedJobFor returns a completed (but not dismissed) job for a tool, if any.
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

// RunningJobFor returns the active job for a tool, if any.
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

// Dismiss removes a completed job from the manager.
func (jm *JobManager) Dismiss(jobID string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if job, ok := jm.jobs[jobID]; ok && job.Done {
		delete(jm.jobs, jobID)
	}
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
