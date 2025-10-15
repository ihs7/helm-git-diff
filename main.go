package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

const (
	defaultBase = "origin/main"
)

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

type Config struct {
	Base                string
	Current             string
	Charts              []string
	ChartDir            string
	ValuesFiles         string
	SetValues           []string
	FailOnDiff          bool
	NoColor             bool
	SkipDependencyBuild bool
	hasDifferences      bool
	useColor            bool
}

func main() {
	config := parseFlags()

	if err := checkGitRepo(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := run(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func checkGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not a git repository (or any of the parent directories)")
	}
	return nil
}

func parseFlags() *Config {
	config := &Config{}

	var setValues multiFlag

	flag.StringVar(&config.Base, "base", defaultBase, "Base git reference to compare from")
	flag.StringVar(&config.Current, "current", "HEAD", "Current git reference to compare to")
	flag.StringVar(&config.ChartDir, "chart-dir", ".", "Directory containing Helm charts")
	flag.StringVar(&config.ValuesFiles, "values", "", "Comma-separated list of values files to use")
	flag.Var(&setValues, "set", "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	flag.BoolVar(&config.FailOnDiff, "fail-on-diff", false, "Exit with code 1 if differences are found")
	flag.BoolVar(&config.NoColor, "no-color", false, "Disable colored output")
	flag.BoolVar(&config.SkipDependencyBuild, "skip-dependency-build", false, "Skip building chart dependencies (use if dependencies are already up to date)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: helm git-diff [flags] [CHART...]\n\n")
		fmt.Fprintf(os.Stderr, "Show Kubernetes resource differences between git commits for Helm charts.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()
	config.Charts = flag.Args()
	config.SetValues = setValues

	if err := detectChartContext(config); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	config.useColor = shouldUseColor(config.NoColor)

	return config
}

func shouldUseColor(noColor bool) bool {
	if noColor {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isTerminal(os.Stdout)
}

func isTerminal(f *os.File) bool {
	fileInfo, err := f.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

func detectChartContext(config *Config) error {
	if len(config.Charts) > 0 {
		return nil
	}

	if _, err := os.Stat("Chart.yaml"); err == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		gitRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return err
		}
		gitRootPath := strings.TrimSpace(string(gitRoot))

		relPath, err := filepath.Rel(gitRootPath, cwd)
		if err != nil {
			return err
		}

		parentPath := filepath.Dir(relPath)
		chartName := filepath.Base(relPath)

		config.ChartDir = parentPath
		config.Charts = []string{chartName}
	}

	return nil
}

func run(config *Config) error {
	if len(config.Charts) == 0 {
		changedCharts, err := detectChangedCharts(config)
		if err != nil {
			return fmt.Errorf("detecting changed charts: %w", err)
		}
		config.Charts = changedCharts

		if len(config.Charts) == 0 {
			fmt.Println("No chart changes detected")
			return nil
		}

		fmt.Printf("Detected changed charts: %s\n\n", strings.Join(config.Charts, ", "))
	}

	for _, chart := range config.Charts {
		if err := diffChart(config, chart); err != nil {
			return fmt.Errorf("diffing chart %s: %w", chart, err)
		}
	}

	if config.FailOnDiff && config.hasDifferences {
		os.Exit(1)
	}

	return nil
}

func detectChangedCharts(config *Config) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", config.Base, config.Current)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running git diff: %w", err)
	}

	changedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	chartSet := make(map[string]bool)

	for _, file := range changedFiles {
		if file == "" {
			continue
		}

		if strings.HasPrefix(file, config.ChartDir+"/") {
			parts := strings.Split(file, "/")
			if len(parts) >= 2 {
				chartName := parts[1]
				chartSet[chartName] = true
			}
		}
	}

	charts := make([]string, 0, len(chartSet))
	for chart := range chartSet {
		charts = append(charts, chart)
	}

	return charts, nil
}

func diffChart(config *Config, chartName string) error {
	chartPath := filepath.Join(config.ChartDir, chartName)

	workdirPath, err := getWorkdirChartPath(chartPath)
	if err != nil {
		return fmt.Errorf("getting workdir chart path: %w", err)
	}

	chartYaml := filepath.Join(workdirPath, "Chart.yaml")
	if _, err := os.Stat(chartYaml); os.IsNotExist(err) {
		return fmt.Errorf("no Chart.yaml found in %s - not a valid Helm chart", chartPath)
	}

	isLibrary, err := isLibraryChart(chartYaml)
	if err != nil {
		return fmt.Errorf("checking chart type: %w", err)
	}
	if isLibrary {
		fmt.Printf("%s: skipped (library chart)\n", chartName)
		return nil
	}

	baseManifest, err := renderChartAtRef(chartPath, config.Base, config.ValuesFiles, config.SetValues, config.SkipDependencyBuild)
	if err != nil {
		return fmt.Errorf("rendering base manifest: %w", err)
	}

	var currentManifest string
	if config.Current == "HEAD" {
		currentManifest, err = renderChartFromWorkdir(workdirPath, config.ValuesFiles, config.SetValues, config.SkipDependencyBuild)
		if err != nil {
			return fmt.Errorf("rendering current manifest: %w", err)
		}
	} else {
		currentManifest, err = renderChartAtRef(chartPath, config.Current, config.ValuesFiles, config.SetValues, config.SkipDependencyBuild)
		if err != nil {
			return fmt.Errorf("rendering current manifest: %w", err)
		}
	}

	if baseManifest == currentManifest {
		fmt.Printf("%s: no changes\n", chartName)
		return nil
	}

	config.hasDifferences = true

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(baseManifest),
		B:        difflib.SplitLines(currentManifest),
		FromFile: fmt.Sprintf("%s (%s)", chartName, config.Base),
		ToFile:   fmt.Sprintf("%s (%s)", chartName, config.Current),
		Context:  3,
	}

	diffText, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return fmt.Errorf("generating diff: %w", err)
	}

	if config.useColor {
		fmt.Print(colorizeDiff(diffText))
	} else {
		fmt.Print(diffText)
	}

	return nil
}

func colorizeDiff(diff string) string {
	const (
		red   = "\033[31m"
		green = "\033[32m"
		cyan  = "\033[36m"
		reset = "\033[0m"
	)

	lines := strings.Split(diff, "\n")
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '-':
			if strings.HasPrefix(line, "---") {
				lines[i] = cyan + line + reset
			} else {
				lines[i] = red + line + reset
			}
		case '+':
			if strings.HasPrefix(line, "+++") {
				lines[i] = cyan + line + reset
			} else {
				lines[i] = green + line + reset
			}
		case '@':
			lines[i] = cyan + line + reset
		}
	}
	return strings.Join(lines, "\n")
}

func getWorkdirChartPath(gitRelativePath string) (string, error) {
	gitRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	gitRootPath := strings.TrimSpace(string(gitRoot))

	if filepath.IsAbs(gitRelativePath) {
		return gitRelativePath, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(gitRelativePath, ".") {
		cwdRelativeToGit, err := filepath.Rel(gitRootPath, cwd)
		if err != nil {
			return "", err
		}
		fullRelativePath := filepath.Join(cwdRelativeToGit, gitRelativePath)
		return filepath.Join(gitRootPath, fullRelativePath), nil
	}

	return filepath.Join(gitRootPath, gitRelativePath), nil
}

func renderChartFromWorkdir(chartPath, valuesFiles string, setValues []string, skipDependencyBuild bool) (string, error) {
	if err := buildDependencies(chartPath, skipDependencyBuild); err != nil {
		return "", fmt.Errorf("building dependencies: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}

	args := []string{"template", "release-name", chartPath}
	if valuesFiles != "" {
		for _, vf := range strings.Split(valuesFiles, ",") {
			valuesPath := strings.TrimSpace(vf)
			if !filepath.IsAbs(valuesPath) {
				valuesPath = filepath.Join(cwd, valuesPath)
			}
			args = append(args, "-f", valuesPath)
		}
	}
	for _, sv := range setValues {
		args = append(args, "--set", sv)
	}

	helmCmd := exec.Command("helm", args...)
	output, err := helmCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("helm template failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("running helm template: %w", err)
	}

	return string(output), nil
}

func renderChartAtRef(chartPath, ref, valuesFiles string, setValues []string, skipDependencyBuild bool) (string, error) {
	tmpDir, err := os.MkdirTemp("", "helm-git-diff-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	gitRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("getting git root: %w", err)
	}
	gitRootPath := strings.TrimSpace(string(gitRoot))

	pathsToExtract, err := getChartPathsToExtract(gitRootPath, ref, chartPath)
	if err != nil {
		return "", fmt.Errorf("determining paths to extract: %w", err)
	}

	args := []string{"archive", ref}
	args = append(args, pathsToExtract...)
	cmd := exec.Command("git", args...)
	cmd.Dir = gitRootPath
	archive, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("archiving chart paths at %s (stderr: %s): %w", ref, string(exitErr.Stderr), err)
		}
		return "", fmt.Errorf("archiving chart paths at %s: %w", ref, err)
	}

	if len(archive) == 0 {
		return "", nil
	}

	extractCmd := exec.Command("tar", "x", "-C", tmpDir)
	extractCmd.Stdin = strings.NewReader(string(archive))
	if err := extractCmd.Run(); err != nil {
		return "", fmt.Errorf("extracting archive: %w", err)
	}

	extractedChartPath := filepath.Join(tmpDir, chartPath)

	if err := buildDependencies(extractedChartPath, skipDependencyBuild); err != nil {
		return "", fmt.Errorf("building dependencies: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}

	helmArgs := []string{"template", "release-name", extractedChartPath}
	if valuesFiles != "" {
		for _, vf := range strings.Split(valuesFiles, ",") {
			valuesPath := strings.TrimSpace(vf)
			if !filepath.IsAbs(valuesPath) {
				valuesPath = filepath.Join(cwd, valuesPath)
			}
			helmArgs = append(helmArgs, "-f", valuesPath)
		}
	}
	for _, sv := range setValues {
		helmArgs = append(helmArgs, "--set", sv)
	}

	helmCmd := exec.Command("helm", helmArgs...)
	output, err := helmCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("helm template failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("running helm template: %w", err)
	}

	return string(output), nil
}

func isLibraryChart(chartYamlPath string) (bool, error) {
	content, err := os.ReadFile(chartYamlPath)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "type:") {
			typeValue := strings.TrimSpace(strings.TrimPrefix(line, "type:"))
			return typeValue == "library", nil
		}
	}
	return false, nil
}

func getChartPathsToExtract(gitRoot, ref, chartPath string) ([]string, error) {
	paths := []string{chartPath}

	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s/Chart.yaml", ref, chartPath))
	cmd.Dir = gitRoot
	output, err := cmd.Output()
	if err != nil {
		return paths, nil
	}

	lines := strings.Split(string(output), "\n")
	inDependencies := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "dependencies:" {
			inDependencies = true
			continue
		}

		if inDependencies {
			if len(trimmed) > 0 && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "name:") && !strings.HasPrefix(trimmed, "version:") && !strings.HasPrefix(trimmed, "repository:") {
				break
			}

			if strings.HasPrefix(trimmed, "repository:") {
				repo := strings.TrimSpace(strings.TrimPrefix(trimmed, "repository:"))
				repo = strings.Trim(repo, "\"'")

				if strings.HasPrefix(repo, "file://") {
					depPath := strings.TrimPrefix(repo, "file://")

					fullPath := filepath.Join(chartPath, depPath)

					cleanedPath := filepath.Clean(fullPath)

					paths = append(paths, cleanedPath)
				}
			}
		}
	}

	return paths, nil
}

func buildDependencies(chartPath string, skipBuild bool) error {
	chartYaml := filepath.Join(chartPath, "Chart.yaml")
	if _, err := os.Stat(chartYaml); os.IsNotExist(err) {
		return nil
	}

	if skipBuild {
		return nil
	}

	if areDependenciesUpToDate(chartPath) {
		return nil
	}

	cmd := exec.Command("helm", "dependency", "build", chartPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm dependency build failed: %s", string(output))
	}

	return nil
}

func areDependenciesUpToDate(chartPath string) bool {
	chartYaml := filepath.Join(chartPath, "Chart.yaml")
	chartLock := filepath.Join(chartPath, "Chart.lock")
	chartsDir := filepath.Join(chartPath, "charts")

	chartYamlInfo, err := os.Stat(chartYaml)
	if err != nil {
		return false
	}

	chartLockInfo, err := os.Stat(chartLock)
	if err != nil {
		return false
	}

	if _, err := os.Stat(chartsDir); err != nil {
		return false
	}

	if chartYamlInfo.ModTime().After(chartLockInfo.ModTime()) {
		return false
	}

	content, err := os.ReadFile(chartYaml)
	if err != nil {
		return false
	}

	hasDependencies := false
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "dependencies:" {
			hasDependencies = true
			break
		}
	}

	if !hasDependencies {
		return true
	}

	entries, err := os.ReadDir(chartsDir)
	if err != nil || len(entries) == 0 {
		return false
	}

	return true
}
