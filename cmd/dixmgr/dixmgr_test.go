package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pierreaubert/dotidx/dix"
)

func TestGenerateStartScript(t *testing.T) {
	tempDir := t.TempDir()
	scriptsDir := filepath.Join(tempDir, "scripts")
	destDir := filepath.Join(tempDir, "bin")

	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("Failed to create scripts dir: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	// Create a minimal reboot.sh.tmpl
	tmplContent := `#!/usr/bin/env bash
set -euo pipefail

echo "dotidx reboot starting at $(date -Is)"

# Restart relay chain services
{{- if .RelayServices }}
echo "Restarting relay chain services..."
{{- range .RelayServices }}
echo "systemctl restart {{ . }}"
systemctl --user restart {{ . }}
{{- end }}
{{- end }}

# Restart parachain services
{{- if .ParachainServices }}
echo "Restarting parachain services..."
{{- range .ParachainServices }}
echo "systemctl restart {{ . }}"
systemctl --user restart {{ . }}
{{- end }}
{{- end }}

echo "dotidx reboot complete at $(date -Is)"
`
	tmplPath := filepath.Join(scriptsDir, "reboot.sh.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	// Create a minimal config
	config := dix.MgrConfig{
		Parachains: map[string]map[string]dix.ParaChainConfig{
			"polkadot": {
				"polkadot": {
					SidecarCount: 1,
				},
				"assetHub": {
					SidecarCount: 1,
				},
			},
		},
	}

	// Test generateStartScript
	if err := generateStartScript(config, destDir, scriptsDir); err != nil {
		t.Fatalf("generateStartScript failed: %v", err)
	}

	startPath := filepath.Join(destDir, "start.sh")
	content, err := os.ReadFile(startPath)
	if err != nil {
		t.Fatalf("Failed to read start.sh: %v", err)
	}

	contentStr := string(content)
	t.Logf("Generated start.sh content:\n%s", contentStr)
	if !contains(contentStr, "dotidx start starting") {
		t.Error("start.sh should contain 'dotidx start starting'")
	}
	if !contains(contentStr, "systemctl --user start") {
		t.Error("start.sh should contain 'systemctl --user start'")
	}
	if !contains(contentStr, "dix-nginx.service") {
		t.Error("start.sh should contain 'dix-nginx.service'")
	}
	if !contains(contentStr, "dixlive dixfe dixbatch dixcron") {
		t.Error("start.sh should contain dix services")
	}

	// Verify order: relay -> parachain -> sidecar -> nginx -> other services
	relayIdx := indexOfSubstring(contentStr, "Start relay chain services")
	parachainIdx := indexOfSubstring(contentStr, "Start parachain services")
	sidecarIdx := indexOfSubstring(contentStr, "Start sidecar services")
	nginxIdx := indexOfSubstring(contentStr, "Start dix-nginx service")

	if relayIdx == -1 || parachainIdx == -1 || sidecarIdx == -1 || nginxIdx == -1 {
		t.Error("start.sh should contain all service sections")
	}
	if !(relayIdx < parachainIdx && parachainIdx < sidecarIdx && sidecarIdx < nginxIdx) {
		t.Errorf("start.sh order should be: relay (%d) -> parachain (%d) -> sidecar (%d) -> nginx (%d)",
			relayIdx, parachainIdx, sidecarIdx, nginxIdx)
	}
}

func TestGenerateStopScript(t *testing.T) {
	tempDir := t.TempDir()
	scriptsDir := filepath.Join(tempDir, "scripts")
	destDir := filepath.Join(tempDir, "bin")

	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("Failed to create scripts dir: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	// Create a minimal reboot.sh.tmpl
	tmplContent := `#!/usr/bin/env bash
set -euo pipefail

echo "dotidx reboot starting at $(date -Is)"

# Restart relay chain services
{{- if .RelayServices }}
echo "Restarting relay chain services..."
{{- range .RelayServices }}
echo "systemctl restart {{ . }}"
systemctl --user restart {{ . }}
{{- end }}
{{- end }}

# Restart parachain services
{{- if .ParachainServices }}
echo "Restarting parachain services..."
{{- range .ParachainServices }}
echo "systemctl restart {{ . }}"
systemctl --user restart {{ . }}
{{- end }}
{{- end }}

echo "dotidx reboot complete at $(date -Is)"
`
	tmplPath := filepath.Join(scriptsDir, "reboot.sh.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	// Create a minimal config
	config := dix.MgrConfig{
		Parachains: map[string]map[string]dix.ParaChainConfig{
			"polkadot": {
				"polkadot": {
					SidecarCount: 1,
				},
				"assetHub": {
					SidecarCount: 1,
				},
			},
		},
	}

	// Test generateStopScript
	if err := generateStopScript(config, destDir, scriptsDir); err != nil {
		t.Fatalf("generateStopScript failed: %v", err)
	}

	stopPath := filepath.Join(destDir, "stop.sh")
	content, err := os.ReadFile(stopPath)
	if err != nil {
		t.Fatalf("Failed to read stop.sh: %v", err)
	}

	contentStr := string(content)
	t.Logf("Generated stop.sh content:\n%s", contentStr)
	if !contains(contentStr, "dotidx stop starting") {
		t.Error("stop.sh should contain 'dotidx stop starting'")
	}
	if !contains(contentStr, "systemctl --user stop") {
		t.Error("stop.sh should contain 'systemctl --user stop'")
	}
	if !contains(contentStr, "dix-nginx.service") {
		t.Error("stop.sh should contain 'dix-nginx.service'")
	}
	if !contains(contentStr, "dixcron dixbatch dixfe dixlive") {
		t.Error("stop.sh should contain dix services in reverse order")
	}

	// Verify reverse order: other services -> nginx -> sidecar -> parachain -> relay
	otherIdx := indexOfSubstring(contentStr, "Stop dixlive")
	nginxIdx := indexOfSubstring(contentStr, "Stop dix-nginx service")
	sidecarIdx := indexOfSubstring(contentStr, "Stop sidecar services")
	parachainIdx := indexOfSubstring(contentStr, "Stop parachain services")
	relayIdx := indexOfSubstring(contentStr, "Stop relay chain services")

	if otherIdx == -1 || nginxIdx == -1 || sidecarIdx == -1 || parachainIdx == -1 || relayIdx == -1 {
		t.Error("stop.sh should contain all service sections")
	}
	if !(otherIdx < nginxIdx && nginxIdx < sidecarIdx && sidecarIdx < parachainIdx && parachainIdx < relayIdx) {
		t.Errorf("stop.sh order should be: other (%d) -> nginx (%d) -> sidecar (%d) -> parachain (%d) -> relay (%d)",
			otherIdx, nginxIdx, sidecarIdx, parachainIdx, relayIdx)
	}
}

func contains(s, substr string) bool {
	return indexOfSubstring(s, substr) != -1
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
