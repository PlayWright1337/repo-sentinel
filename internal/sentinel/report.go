package sentinel

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultOutputFileName = "REPO_REPORT.md"
	fileModeReadable      = 0o644
	directoryMode         = 0o755
	bytesInKilobyte       = 1024
	tableLimit            = 12
)

var ErrRepositoryPathRequired = errors.New("repository path is required")

type Options struct {
	RootDirectory            string
	OutputPath               string
	IncludeHiddenDirectories bool
}

type Report struct {
	RootDirectory string
	GeneratedAt   time.Time
	TotalFiles    int
	TotalBytes    int64
	LanguageStats []LanguageStat
	LargestFiles  []FileStat
	RecentFiles   []FileStat
}

type LanguageStat struct {
	Name  string
	Files int
	Bytes int64
}

type FileStat struct {
	Path       string
	Bytes      int64
	ModifiedAt time.Time
}

func DefaultOptions(rootDirectory string) Options {
	return Options{
		RootDirectory:            rootDirectory,
		OutputPath:               defaultOutputFileName,
		IncludeHiddenDirectories: false,
	}
}

func BuildReport(options Options) (Report, error) {
	if strings.TrimSpace(options.RootDirectory) == "" {
		return Report{}, ErrRepositoryPathRequired
	}

	rootInfo, err := os.Stat(options.RootDirectory)
	if err != nil {
		return Report{}, fmt.Errorf("failed to inspect repository path: %w", err)
	}

	if !rootInfo.IsDir() {
		return Report{}, fmt.Errorf("repository path must be a directory: %s", options.RootDirectory)
	}

	languageStatsByName := make(map[string]LanguageStat)
	fileStats := make([]FileStat, 0)
	report := Report{
		RootDirectory: options.RootDirectory,
		GeneratedAt:   time.Now().UTC(),
	}

	walkError := filepath.WalkDir(options.RootDirectory, func(path string, directoryEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("failed to walk path %s: %w", path, walkErr)
		}

		if shouldSkipPath(options, path, directoryEntry) {
			return filepath.SkipDir
		}

		if directoryEntry.IsDir() {
			return nil
		}

		fileInfo, err := directoryEntry.Info()
		if err != nil {
			return fmt.Errorf("failed to inspect file %s: %w", path, err)
		}

		relativePath, err := filepath.Rel(options.RootDirectory, path)
		if err != nil {
			return fmt.Errorf("failed to build relative path for %s: %w", path, err)
		}

		fileStat := FileStat{
			Path:       filepath.ToSlash(relativePath),
			Bytes:      fileInfo.Size(),
			ModifiedAt: fileInfo.ModTime().UTC(),
		}

		languageName := languageNameForPath(path)
		languageStat := languageStatsByName[languageName]
		languageStat.Name = languageName
		languageStat.Files++
		languageStat.Bytes += fileInfo.Size()
		languageStatsByName[languageName] = languageStat

		fileStats = append(fileStats, fileStat)
		report.TotalFiles++
		report.TotalBytes += fileInfo.Size()

		return nil
	})
	if walkError != nil {
		return Report{}, walkError
	}

	report.LanguageStats = sortedLanguageStats(languageStatsByName)
	report.LargestFiles = largestFiles(fileStats)
	report.RecentFiles = recentFiles(fileStats)

	return report, nil
}

func WriteReport(outputPath string, report Report) error {
	outputDirectory := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDirectory, directoryMode); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	reportContent := RenderMarkdown(report)
	if err := os.WriteFile(outputPath, []byte(reportContent), fileModeReadable); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

func RenderMarkdown(report Report) string {
	var content bytes.Buffer

	fmt.Fprintf(&content, "# Repository Report\n\n")
	fmt.Fprintf(&content, "Generated: `%s`\n\n", report.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&content, "Path: `%s`\n\n", filepath.ToSlash(report.RootDirectory))
	fmt.Fprintf(&content, "Files: `%d`\n\n", report.TotalFiles)
	fmt.Fprintf(&content, "Size: `%s`\n\n", formatBytes(report.TotalBytes))

	writeLanguageTable(&content, report.LanguageStats)
	writeFileTable(&content, "Largest files", report.LargestFiles, true)
	writeFileTable(&content, "Recently changed", report.RecentFiles, false)

	return content.String()
}

func shouldSkipPath(options Options, path string, directoryEntry fs.DirEntry) bool {
	if !directoryEntry.IsDir() {
		return false
	}

	directoryName := directoryEntry.Name()
	if path == options.RootDirectory {
		return false
	}

	if directoryName == ".git" {
		return true
	}

	return !options.IncludeHiddenDirectories && strings.HasPrefix(directoryName, ".")
}

func sortedLanguageStats(languageStatsByName map[string]LanguageStat) []LanguageStat {
	languageStats := make([]LanguageStat, 0, len(languageStatsByName))
	for _, languageStat := range languageStatsByName {
		languageStats = append(languageStats, languageStat)
	}

	sort.Slice(languageStats, func(firstIndex int, secondIndex int) bool {
		firstLanguageStat := languageStats[firstIndex]
		secondLanguageStat := languageStats[secondIndex]
		if firstLanguageStat.Bytes == secondLanguageStat.Bytes {
			return firstLanguageStat.Name < secondLanguageStat.Name
		}
		return firstLanguageStat.Bytes > secondLanguageStat.Bytes
	})

	return limitLanguageStats(languageStats)
}

func limitLanguageStats(languageStats []LanguageStat) []LanguageStat {
	if len(languageStats) <= tableLimit {
		return languageStats
	}
	return languageStats[:tableLimit]
}

func largestFiles(fileStats []FileStat) []FileStat {
	sortedFiles := append([]FileStat(nil), fileStats...)
	sort.Slice(sortedFiles, func(firstIndex int, secondIndex int) bool {
		firstFileStat := sortedFiles[firstIndex]
		secondFileStat := sortedFiles[secondIndex]
		if firstFileStat.Bytes == secondFileStat.Bytes {
			return firstFileStat.Path < secondFileStat.Path
		}
		return firstFileStat.Bytes > secondFileStat.Bytes
	})
	return limitFileStats(sortedFiles)
}

func recentFiles(fileStats []FileStat) []FileStat {
	sortedFiles := append([]FileStat(nil), fileStats...)
	sort.Slice(sortedFiles, func(firstIndex int, secondIndex int) bool {
		firstFileStat := sortedFiles[firstIndex]
		secondFileStat := sortedFiles[secondIndex]
		if firstFileStat.ModifiedAt.Equal(secondFileStat.ModifiedAt) {
			return firstFileStat.Path < secondFileStat.Path
		}
		return firstFileStat.ModifiedAt.After(secondFileStat.ModifiedAt)
	})
	return limitFileStats(sortedFiles)
}

func limitFileStats(fileStats []FileStat) []FileStat {
	if len(fileStats) <= tableLimit {
		return fileStats
	}
	return fileStats[:tableLimit]
}

func languageNameForPath(path string) string {
	extension := strings.ToLower(filepath.Ext(path))
	languageName, exists := languageNamesByExtension[extension]
	if exists {
		return languageName
	}
	if extension == "" {
		return "Other"
	}
	return strings.TrimPrefix(extension, ".")
}

func writeLanguageTable(content *bytes.Buffer, languageStats []LanguageStat) {
	fmt.Fprintf(content, "## Languages\n\n")
	fmt.Fprintf(content, "| Language | Files | Size |\n")
	fmt.Fprintf(content, "| --- | ---: | ---: |\n")

	if len(languageStats) == 0 {
		fmt.Fprintf(content, "| None | 0 | 0 B |\n\n")
		return
	}

	for _, languageStat := range languageStats {
		fmt.Fprintf(content, "| %s | %d | %s |\n", languageStat.Name, languageStat.Files, formatBytes(languageStat.Bytes))
	}

	fmt.Fprintf(content, "\n")
}

func writeFileTable(content *bytes.Buffer, title string, fileStats []FileStat, includeSize bool) {
	fmt.Fprintf(content, "## %s\n\n", title)

	if includeSize {
		fmt.Fprintf(content, "| File | Size | Modified |\n")
		fmt.Fprintf(content, "| --- | ---: | --- |\n")
	} else {
		fmt.Fprintf(content, "| File | Modified | Size |\n")
		fmt.Fprintf(content, "| --- | --- | ---: |\n")
	}

	if len(fileStats) == 0 {
		if includeSize {
			fmt.Fprintf(content, "| None | 0 B | - |\n\n")
			return
		}
		fmt.Fprintf(content, "| None | - | 0 B |\n\n")
		return
	}

	for _, fileStat := range fileStats {
		if includeSize {
			fmt.Fprintf(content, "| `%s` | %s | %s |\n", fileStat.Path, formatBytes(fileStat.Bytes), fileStat.ModifiedAt.Format(time.RFC3339))
			continue
		}
		fmt.Fprintf(content, "| `%s` | %s | %s |\n", fileStat.Path, fileStat.ModifiedAt.Format(time.RFC3339), formatBytes(fileStat.Bytes))
	}

	fmt.Fprintf(content, "\n")
}

func formatBytes(bytesCount int64) string {
	if bytesCount < bytesInKilobyte {
		return fmt.Sprintf("%d B", bytesCount)
	}

	kilobytes := float64(bytesCount) / bytesInKilobyte
	return fmt.Sprintf("%.1f KB", kilobytes)
}
