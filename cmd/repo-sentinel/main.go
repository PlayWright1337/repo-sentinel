package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"repo-sentinel/internal/sentinel"
)

const failureExitCode = 1

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(failureExitCode)
	}
}

func run(arguments []string) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to detect current directory: %w", err)
	}

	options := sentinel.DefaultOptions(workingDirectory)

	commandFlags := flag.NewFlagSet("repo-sentinel", flag.ContinueOnError)
	commandFlags.SetOutput(os.Stderr)
	commandFlags.StringVar(&options.RootDirectory, "path", options.RootDirectory, "repository path")
	commandFlags.StringVar(&options.OutputPath, "out", options.OutputPath, "report output path")
	commandFlags.BoolVar(&options.IncludeHiddenDirectories, "hidden", options.IncludeHiddenDirectories, "include hidden directories")

	if err := commandFlags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	options.RootDirectory, err = filepath.Abs(options.RootDirectory)
	if err != nil {
		return fmt.Errorf("failed to resolve repository path: %w", err)
	}

	if !filepath.IsAbs(options.OutputPath) {
		options.OutputPath = filepath.Join(options.RootDirectory, options.OutputPath)
	}

	report, err := sentinel.BuildReport(options)
	if err != nil {
		return err
	}

	if err := sentinel.WriteReport(options.OutputPath, report); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "report written to %s\n", options.OutputPath)
	return nil
}
