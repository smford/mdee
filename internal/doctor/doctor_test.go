package doctor

import (
	"strings"
	"testing"
)

func TestRunDiagnostics(t *testing.T) {
	report := RunDiagnostics()
	if !strings.Contains(report, "Terminal Diagnostic") {
		t.Errorf("expected diagnostic header in report")
	}
	if !strings.Contains(report, "OSC 1337") {
		t.Errorf("expected OSC 1337 check in report")
	}
	if !strings.Contains(report, "OSC 8") {
		t.Errorf("expected OSC 8 check in report")
	}
	if !strings.Contains(report, "Mermaid Protocol") {
		t.Errorf("expected Mermaid Protocol check in report")
	}
	if !strings.Contains(report, "Mermaid CLI") {
		t.Errorf("expected Mermaid CLI check in report")
	}
	if !strings.Contains(report, "Mermaid Text Engine") {
		t.Errorf("expected Mermaid Text Engine check in report")
	}
	if !strings.Contains(report, "Terminal Emulator") {
		t.Errorf("expected Terminal Emulator check in report")
	}
	if !strings.Contains(report, "Active Graphics Protocol") {
		t.Errorf("expected Active Graphics Protocol check in report")
	}
	if !strings.Contains(report, "Kitty Graphics") {
		t.Errorf("expected Kitty Graphics check in report")
	}
	if !strings.Contains(report, "DEC Sixel Graphics") {
		t.Errorf("expected DEC Sixel Graphics check in report")
	}
	if !strings.Contains(report, "Mermaid Diagram Rendering Test") {
		t.Errorf("expected Mermaid Diagram Rendering Test in report")
	}
}
