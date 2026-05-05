# repo-sentinel

Small Go CLI that scans a repository and writes a Markdown report with language stats, largest files, and recently changed files.

## Run

```bash
go run ./cmd/repo-sentinel
```

```bash
go run ./cmd/repo-sentinel -path . -out REPO_REPORT.md
```

## Test

```bash
go test ./...
```
