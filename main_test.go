package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseFlags(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"cmd", "--base", "main", "--current", "feature", "--chart-dir", "mychart", "chart1", "chart2"}

	config := parseFlags()

	if config.Base != "main" {
		t.Errorf("expected Base to be 'main', got '%s'", config.Base)
	}
	if config.Current != "feature" {
		t.Errorf("expected Current to be 'feature', got '%s'", config.Current)
	}
	if config.ChartDir != "mychart" {
		t.Errorf("expected ChartDir to be 'mychart', got '%s'", config.ChartDir)
	}
	if len(config.Charts) != 2 {
		t.Errorf("expected 2 charts, got %d", len(config.Charts))
	}
}

func TestDetectChangedCharts(t *testing.T) {
	if !isGitRepo() {
		t.Skip("skipping test: not in a git repository")
	}

	config := &Config{
		Base:     "HEAD",
		Current:  "HEAD",
		ChartDir: "charts",
	}

	charts, err := detectChangedCharts(config)
	if err != nil {
		t.Fatalf("detectChangedCharts failed: %v", err)
	}

	if len(charts) != 0 {
		t.Logf("detected charts (comparing HEAD to HEAD, should be empty): %v", charts)
	}
}

func TestRenderChartAtRef(t *testing.T) {
	if !isGitRepo() {
		t.Skip("skipping test: not in a git repository")
	}

	tmpDir := t.TempDir()
	chartPath := filepath.Join(tmpDir, "testchart")

	if err := os.MkdirAll(filepath.Join(chartPath, "templates"), 0755); err != nil {
		t.Fatal(err)
	}

	chartYAML := `apiVersion: v2
name: testchart
version: 0.1.0
`
	if err := os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte(chartYAML), 0644); err != nil {
		t.Fatal(err)
	}

	template := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value
`
	if err := os.WriteFile(filepath.Join(chartPath, "templates", "configmap.yaml"), []byte(template), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(origDir)
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	manifest, err := renderChartAtRef("testchart", "HEAD", "", nil)
	if err != nil {
		t.Fatalf("renderChartAtRef failed: %v", err)
	}

	if manifest == "" {
		t.Error("expected non-empty manifest")
	}

	if !contains(manifest, "ConfigMap") {
		t.Error("expected manifest to contain 'ConfigMap'")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != substr && len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}
