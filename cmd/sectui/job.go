package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/orlandobianco/SecTUI/internal/core"
	"github.com/orlandobianco/SecTUI/internal/tools"
	"github.com/spf13/cobra"
)

// jobMeta mirrors the Job struct for JSON serialization.
type jobMeta struct {
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
}

func newJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "job",
		Short:  "Background job management (internal)",
		Hidden: true,
	}

	cmd.AddCommand(newJobExecCmd())
	return cmd
}

func newJobExecCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "exec <tool:action>",
		Short: "Execute a tool action with output to file (internal)",
		Args:  cobra.ExactArgs(1),
		RunE:  runJobExec,
	}
}

func runJobExec(_ *cobra.Command, args []string) error {
	parts := strings.SplitN(args[0], ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid format: expected tool:action, got %q", args[0])
	}
	toolID := parts[0]
	actionID := parts[1]

	// Detect platform and find the tool.
	platform := core.DetectPlatform()
	allTools := tools.ApplicableTools(platform)

	var tm core.ToolManager
	for _, t := range allTools {
		if t.ID() == toolID {
			if m, ok := t.(core.ToolManager); ok {
				tm = m
			}
			break
		}
	}

	if tm == nil {
		return fmt.Errorf("tool %q not found or not a ToolManager", toolID)
	}

	// Resolve paths.
	dir, err := jobsDirPath()
	if err != nil {
		return err
	}

	safeID := strings.ReplaceAll(args[0], ":", "_")
	metaPath := filepath.Join(dir, safeID+".json")
	logPath := filepath.Join(dir, safeID+".log")

	// Read existing meta (written by the parent TUI process).
	meta := jobMeta{
		ID:        args[0],
		ToolID:    toolID,
		ActionID:  actionID,
		PID:       os.Getpid(),
		StartedAt: time.Now(),
	}
	if data, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(data, &meta)
	}
	// Update PID to our actual PID (parent wrote its child PID which should match).
	meta.PID = os.Getpid()
	saveMeta(metaPath, &meta)

	// Open log file for streaming output.
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logFile.Close()

	// Execute the action.
	var result core.ActionResult

	if se, ok := tm.(core.StreamingExecutor); ok {
		result = se.ExecuteActionToFile(actionID, logFile)
	} else {
		// Fallback: run synchronously and write output to log.
		result = tm.ExecuteAction(actionID)
		fmt.Fprintln(logFile, result.Message)
	}

	// Update meta with results.
	meta.Done = true
	meta.Success = result.Success
	meta.Message = result.Message
	if result.Success {
		meta.ExitCode = 0
	} else {
		meta.ExitCode = 1
	}
	saveMeta(metaPath, &meta)

	return nil
}

func saveMeta(path string, meta *jobMeta) {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func jobsDirPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "sectui", "jobs")
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	return p, nil
}
