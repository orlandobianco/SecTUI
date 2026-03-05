package modules

import (
	"strings"
	"testing"

	"github.com/orlandobianco/SecTUI/internal/core"
)

func TestKeyFromSysctlLine(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"net.ipv4.conf.all.rp_filter = 1", "net.ipv4.conf.all.rp_filter"},
		{"net.ipv4.conf.all.rp_filter=1", "net.ipv4.conf.all.rp_filter"},
		{"kernel.randomize_va_space = 2", "kernel.randomize_va_space"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := keyFromSysctlLine(tt.line)
			if got != tt.want {
				t.Errorf("keyFromSysctlLine(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestUpsertSysctlParam_NewParam(t *testing.T) {
	content := "# Header\nnet.ipv4.tcp_syncookies = 1\n"
	result := upsertSysctlParam(content, "kernel.randomize_va_space", "2")

	if !strings.Contains(result, "kernel.randomize_va_space = 2") {
		t.Errorf("new param not appended:\n%s", result)
	}
	// Original should be preserved.
	if !strings.Contains(result, "net.ipv4.tcp_syncookies = 1") {
		t.Error("original param was lost")
	}
}

func TestUpsertSysctlParam_ReplaceExisting(t *testing.T) {
	content := "net.ipv4.tcp_syncookies = 0\nkernel.randomize_va_space = 1\n"
	result := upsertSysctlParam(content, "net.ipv4.tcp_syncookies", "1")

	if !strings.Contains(result, "net.ipv4.tcp_syncookies = 1") {
		t.Errorf("param not replaced:\n%s", result)
	}
	// Should NOT have the old value.
	if strings.Contains(result, "net.ipv4.tcp_syncookies = 0") {
		t.Error("old value still present")
	}
	// Other params should be preserved.
	if !strings.Contains(result, "kernel.randomize_va_space = 1") {
		t.Error("other param was lost")
	}
}

func TestUpsertSysctlParam_UncommentsExisting(t *testing.T) {
	content := "# net.ipv4.tcp_syncookies = 0\n"
	result := upsertSysctlParam(content, "net.ipv4.tcp_syncookies", "1")

	if !strings.Contains(result, "net.ipv4.tcp_syncookies = 1") {
		t.Errorf("commented param not replaced:\n%s", result)
	}
}

func TestUpsertSysctlParam_EmptyContent(t *testing.T) {
	result := upsertSysctlParam("", "kernel.randomize_va_space", "2")
	if !strings.Contains(result, "kernel.randomize_va_space = 2") {
		t.Errorf("param not added to empty content:\n%s", result)
	}
}

func TestKernelModule_ID(t *testing.T) {
	m := NewKernelModule()
	if m.ID() != "kernel" {
		t.Errorf("ID() = %q, want kernel", m.ID())
	}
}

func TestKernelModule_IsApplicable(t *testing.T) {
	m := NewKernelModule()

	linux := &core.PlatformInfo{OS: core.OSLinux}
	darwin := &core.PlatformInfo{OS: core.OSDarwin}

	if !m.IsApplicable(linux) {
		t.Error("should be applicable on Linux")
	}
	if m.IsApplicable(darwin) {
		t.Error("should not be applicable on macOS")
	}
	if m.IsApplicable(nil) {
		t.Error("should not be applicable with nil platform")
	}
}

func TestKernelModule_AvailableFixes(t *testing.T) {
	m := NewKernelModule()
	fixes := m.AvailableFixes()
	if len(fixes) != 13 {
		t.Errorf("AvailableFixes() len = %d, want 13", len(fixes))
	}
	for _, f := range fixes {
		if f.ID == "" {
			t.Error("fix has empty ID")
		}
		if f.FindingID == "" {
			t.Error("fix has empty FindingID")
		}
	}
}

func TestKernelModule_PreviewFix_UnknownFix(t *testing.T) {
	m := NewKernelModule()
	_, err := m.PreviewFix("nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown fix ID")
	}
}

func TestKernelModule_ApplyFix_UnknownFix(t *testing.T) {
	m := NewKernelModule()
	_, err := m.ApplyFix("nonexistent", &core.ApplyContext{DryRun: true})
	if err == nil {
		t.Error("expected error for unknown fix ID")
	}
}

func TestKernelModule_ApplyFix_DryRun(t *testing.T) {
	m := NewKernelModule()
	result, err := m.ApplyFix("fix-kern-001", &core.ApplyContext{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run ApplyFix failed: %v", err)
	}
	if !result.Success {
		t.Error("dry-run should succeed")
	}
	if !strings.Contains(result.Message, "dry-run") {
		t.Errorf("dry-run message missing 'dry-run': %q", result.Message)
	}
}

func TestKernelModule_FindCheckByFixID(t *testing.T) {
	m := NewKernelModule()

	check := m.findCheckByFixID("fix-kern-007")
	if check == nil {
		t.Fatal("findCheckByFixID returned nil for valid ID")
	}
	if check.param != "kernel.randomize_va_space" {
		t.Errorf("param = %q, want kernel.randomize_va_space", check.param)
	}

	if m.findCheckByFixID("nonexistent") != nil {
		t.Error("findCheckByFixID should return nil for invalid ID")
	}
}
