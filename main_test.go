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

	manifest, err := renderChartAtRef("testchart", "HEAD", "", nil, false)
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

func TestAreDependenciesUpToDate(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(string) error
		expected bool
	}{
		{
			name: "missing Chart.yaml",
			setup: func(chartPath string) error {
				return nil
			},
			expected: false,
		},
		{
			name: "missing Chart.lock",
			setup: func(chartPath string) error {
				return os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte("apiVersion: v2\nname: test\n"), 0644)
			},
			expected: false,
		},
		{
			name: "missing charts directory",
			setup: func(chartPath string) error {
				if err := os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte("apiVersion: v2\nname: test\n"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(chartPath, "Chart.lock"), []byte("dependencies: []\n"), 0644)
			},
			expected: false,
		},
		{
			name: "Chart.yaml newer than Chart.lock",
			setup: func(chartPath string) error {
				if err := os.WriteFile(filepath.Join(chartPath, "Chart.lock"), []byte("dependencies: []\n"), 0644); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Join(chartPath, "charts"), 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte("apiVersion: v2\nname: test\ndependencies:\n- name: foo\n"), 0644)
			},
			expected: false,
		},
		{
			name: "no dependencies in Chart.yaml",
			setup: func(chartPath string) error {
				if err := os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte("apiVersion: v2\nname: test\n"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(chartPath, "Chart.lock"), []byte("dependencies: []\n"), 0644); err != nil {
					return err
				}
				return os.MkdirAll(filepath.Join(chartPath, "charts"), 0755)
			},
			expected: true,
		},
		{
			name: "dependencies up to date",
			setup: func(chartPath string) error {
				if err := os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte("apiVersion: v2\nname: test\ndependencies:\n- name: foo\n"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(chartPath, "Chart.lock"), []byte("dependencies: []\n"), 0644); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Join(chartPath, "charts"), 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(chartPath, "charts", "foo-1.0.0.tgz"), []byte("dummy"), 0644)
			},
			expected: true,
		},
		{
			name: "empty charts directory",
			setup: func(chartPath string) error {
				if err := os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte("apiVersion: v2\nname: test\ndependencies:\n- name: foo\n"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(chartPath, "Chart.lock"), []byte("dependencies: []\n"), 0644); err != nil {
					return err
				}
				return os.MkdirAll(filepath.Join(chartPath, "charts"), 0755)
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			chartPath := filepath.Join(tmpDir, "testchart")
			if err := os.MkdirAll(chartPath, 0755); err != nil {
				t.Fatal(err)
			}

			if err := tt.setup(chartPath); err != nil {
				t.Fatal(err)
			}

			result := areDependenciesUpToDate(chartPath)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBuildDependenciesWithSkip(t *testing.T) {
	tmpDir := t.TempDir()
	chartPath := filepath.Join(tmpDir, "testchart")
	if err := os.MkdirAll(chartPath, 0755); err != nil {
		t.Fatal(err)
	}

	chartYAML := `apiVersion: v2
name: testchart
version: 0.1.0
dependencies:
- name: common
  version: "1.0.0"
  repository: https://charts.example.com
`
	if err := os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte(chartYAML), 0644); err != nil {
		t.Fatal(err)
	}

	err := buildDependencies(chartPath, true)
	if err != nil {
		t.Errorf("buildDependencies with skip=true should not fail: %v", err)
	}

	if _, err := os.Stat(filepath.Join(chartPath, "Chart.lock")); err == nil {
		t.Error("Chart.lock should not be created when skip=true")
	}
}

func TestBuildDependenciesNoDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	chartPath := filepath.Join(tmpDir, "testchart")
	if err := os.MkdirAll(chartPath, 0755); err != nil {
		t.Fatal(err)
	}

	chartYAML := `apiVersion: v2
name: testchart
version: 0.1.0
`
	if err := os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte(chartYAML), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(chartPath, "Chart.lock"), []byte("dependencies: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(chartPath, "charts"), 0755); err != nil {
		t.Fatal(err)
	}

	err := buildDependencies(chartPath, false)
	if err != nil {
		t.Errorf("buildDependencies should succeed for chart with no dependencies: %v", err)
	}
}

func TestConfigSkipDependencyBuildField(t *testing.T) {
	config := &Config{
		SkipDependencyBuild: true,
	}

	if !config.SkipDependencyBuild {
		t.Error("expected SkipDependencyBuild to be true")
	}

	config2 := &Config{}
	if config2.SkipDependencyBuild {
		t.Error("expected SkipDependencyBuild to be false by default")
	}
}

func TestRenderChartWithSkipDependencies(t *testing.T) {
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

	manifest, err := renderChartAtRef("testchart", "HEAD", "", nil, true)
	if err != nil {
		t.Fatalf("renderChartAtRef with skip=true failed: %v", err)
	}

	if manifest == "" {
		t.Error("expected non-empty manifest")
	}

	if !contains(manifest, "ConfigMap") {
		t.Error("expected manifest to contain 'ConfigMap'")
	}
}
