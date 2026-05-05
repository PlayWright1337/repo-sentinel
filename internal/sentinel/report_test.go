package sentinel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportCountsVisibleFilesAndSkipsGitDirectory(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	writeTestFile(t, filepath.Join(rootDirectory, "main.go"), "package main\n")
	writeTestFile(t, filepath.Join(rootDirectory, "README.md"), "# Example\n")
	writeTestFile(t, filepath.Join(rootDirectory, ".git", "config"), "ignored")

	report, err := BuildReport(DefaultOptions(rootDirectory))
	if err != nil {
		t.Fatalf("BuildReport returned error: %v", err)
	}

	if report.TotalFiles != 2 {
		t.Fatalf("TotalFiles = %d, want 2", report.TotalFiles)
	}

	if len(report.LanguageStats) != 2 {
		t.Fatalf("LanguageStats length = %d, want 2", len(report.LanguageStats))
	}
}

func TestBuildReportIncludesHiddenDirectoriesWhenEnabled(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	writeTestFile(t, filepath.Join(rootDirectory, ".github", "workflow.yml"), "name: checks\n")

	options := DefaultOptions(rootDirectory)
	options.IncludeHiddenDirectories = true

	report, err := BuildReport(options)
	if err != nil {
		t.Fatalf("BuildReport returned error: %v", err)
	}

	if report.TotalFiles != 1 {
		t.Fatalf("TotalFiles = %d, want 1", report.TotalFiles)
	}
}

func TestRenderMarkdownContainsSummaryAndTables(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	writeTestFile(t, filepath.Join(rootDirectory, "main.go"), "package main\n")

	report, err := BuildReport(DefaultOptions(rootDirectory))
	if err != nil {
		t.Fatalf("BuildReport returned error: %v", err)
	}

	markdown := RenderMarkdown(report)

	expectedFragments := []string{
		"# Repository Report",
		"## Languages",
		"## Largest files",
		"## Recently changed",
		"Go",
	}

	for _, expectedFragment := range expectedFragments {
		if !strings.Contains(markdown, expectedFragment) {
			t.Fatalf("markdown does not contain %q", expectedFragment)
		}
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), directoryMode); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), fileModeReadable); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}
