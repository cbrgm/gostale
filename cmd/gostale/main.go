package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	args "github.com/alexflint/go-arg"
)

// Args defines the CLI arguments for gostale.
type Args struct {
	Path              string `arg:"positional" help:"Path or pattern (e.g. ./...) to scan for Go files"`
	Today             string `arg:"--today" help:"Override today's date in DD-MM-YYYY format"`
	Exclude           string `arg:"--exclude" help:"Comma-separated list of directories to exclude"`
	FailOnExpired     bool   `arg:"--fail-on-expired" help:"Exit with code 1 if expired annotations are found"`
	LogLevel          string `arg:"--log-level" help:"Log level: debug, info, warn, error (default: info)"`
	DefaultExpiryDays int    `arg:"--default-expiry-days" help:"Default expiration offset in days if not set in comment (default: 90)"`
	DateFormat        string `arg:"--date-format" help:"Date format for annotations (default: 02-01-2006)"`
}

// Annotation represents a parsed gostale comment in a Go file.
type Annotation struct {
	File        string
	Line        int
	Declaration string
	WarnDate    time.Time
	ExpireDate  *time.Time
	Message     string
}

var argsData Args

func main() {
	args.MustParse(&argsData)

	if argsData.Path == "" {
		argsData.Path = "."
	}
	if argsData.DefaultExpiryDays <= 0 {
		argsData.DefaultExpiryDays = 90
	}
	if argsData.DateFormat == "" {
		argsData.DateFormat = "02-01-2006"
	}

	log := setupLogger(argsData.LogLevel)
	log.Debug("Logger initialized", slog.String("level", argsData.LogLevel))

	today := resolveDate(argsData.Today, argsData.DateFormat, log)
	log.Debug("Parsed today date", slog.Any("today", today))
	excludes := parseExcludes(argsData.Exclude)
	log.Debug("Parsed excludes", slog.Any("excludes", excludes))

	files, err := collectGoFiles(argsData.Path, excludes, log)
	if err != nil {
		log.Error("Failed to collect files", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Debug("Go files collected", slog.Int("count", len(files)))

	var annotations []Annotation
	for _, file := range files {
		log.Debug("Scanning file", slog.String("file", file))
		found, err := scanFile(file, log)
		if err != nil {
			log.Warn("Skipping file", slog.String("file", file), slog.String("error", err.Error()))
			continue
		}
		annotations = append(annotations, found...)
	}

	log.Debug("Total annotations found", slog.Int("count", len(annotations)))
	expired := reportAnnotations(annotations, today, log)

	if expired && argsData.FailOnExpired {
		log.Error("Expired code found", slog.String("exit", "1"))
		os.Exit(1)
	}
}

func resolveDate(input, format string, log *slog.Logger) time.Time {
	dateStr := input
	if dateStr == "" {
		dateStr = os.Getenv("GOSTALE_DATE")
	}
	if dateStr != "" {
		t, err := time.Parse(format, dateStr)
		if err != nil {
			log.Error("Invalid date format for --today or GOSTALE_DATE",
				slog.String("got", dateStr),
				slog.String("expected_format", format),
				slog.String("hint", "Use --date-format to specify the format"))
			os.Exit(1)
		}
		log.Debug("Date override used", slog.String("date", t.Format(time.RFC3339)))
		return t
	}
	now := time.Now()
	log.Debug("Using current date", slog.String("date", now.Format(time.RFC3339)))
	return now
}

// scanFile parses a Go source file and returns all extracted stale annotations.
func scanFile(path string, log *slog.Logger) ([]Annotation, error) {
	var results []Annotation
	fs := token.NewFileSet()

	node, err := parser.ParseFile(fs, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Match TODO or FIXME comments (case-insensitive) with optional (user) and key-value fields
	re := regexp.MustCompile(`(?i)//\s*(todo|fixme)(\([^)]*\))?:\s*stale:\s*(\d{2}-\d{2}-\d{4})(?:\s+expires:\s*(\d{2}-\d{2}-\d{4}))?(?:\s+(.*))?`)

	for _, cg := range node.Comments {
		for _, c := range cg.List {
			match := re.FindStringSubmatch(c.Text)
			if match == nil {
				continue
			}

			warn, err := time.Parse(argsData.DateFormat, match[3])
			if err != nil {
				log.Warn("Invalid stale date format", slog.String("input", match[3]), slog.String("expected", argsData.DateFormat))
				continue
			}

			var expire *time.Time
			if match[4] != "" {
				t, err := time.Parse(argsData.DateFormat, match[4])
				if err != nil {
					log.Warn("Invalid expiration date format", slog.String("input", match[4]), slog.String("expected", argsData.DateFormat))
					continue
				}
				expire = &t
			} else {
				e := warn.AddDate(0, 0, argsData.DefaultExpiryDays)
				expire = &e
			}

			msg := strings.TrimSpace(match[5])
			line := fs.Position(c.Pos()).Line
			decl := findDecl(c.Pos(), node)

			log.Debug("Annotation found", slog.String("file", path), slog.Int("line", line), slog.String("decl", decl))

			results = append(results, Annotation{
				File:        path,
				Line:        line,
				Declaration: decl,
				WarnDate:    warn,
				ExpireDate:  expire,
				Message:     msg,
			})
		}
	}
	return results, nil
}

func stringToLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func setupLogger(level string) *slog.Logger {
	logLevel := stringToLogLevel(level)
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	return slog.New(handler)
}

func parseExcludes(raw string) map[string]bool {
	ex := map[string]bool{}
	for _, e := range strings.Split(raw, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			ex[e] = true
		}
	}
	return ex
}

func collectGoFiles(pattern string, excludes map[string]bool, log *slog.Logger) ([]string, error) {
	var files []string

	if strings.HasSuffix(pattern, "...") {
		cmd := exec.Command("go", "list", "-f", `{{.Dir}}`, pattern)
		out, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			dir := scanner.Text()
			skip := false
			for e := range excludes {
				if strings.Contains(dir, e) {
					log.Debug("Excluding directory", slog.String("dir", dir))
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && strings.HasSuffix(path, ".go") {
					files = append(files, path)
				}
				return nil
			})
		}
		return files, nil
	}

	info, err := os.Stat(pattern)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return collectGoFiles(pattern+"/...", excludes, log)
	}
	return []string{pattern}, nil
}

func findDecl(pos token.Pos, node *ast.File) string {
	for _, d := range node.Decls {
		if pos >= d.Pos() && pos <= d.End() {
			switch decl := d.(type) {
			case *ast.FuncDecl:
				return decl.Name.Name
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						return s.Name.Name
					case *ast.ValueSpec:
						if len(s.Names) > 0 {
							return s.Names[0].Name
						}
					}
				}
			}
		}
	}
	for _, d := range node.Decls {
		if pos < d.Pos() {
			switch decl := d.(type) {
			case *ast.FuncDecl:
				return decl.Name.Name
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						return s.Name.Name
					case *ast.ValueSpec:
						if len(s.Names) > 0 {
							return s.Names[0].Name
						}
					}
				}
			}
		}
	}
	return "<unknown>"
}

func reportAnnotations(all []Annotation, today time.Time, log *slog.Logger) bool {
	expired := false
	for _, a := range all {
		if today.Before(a.WarnDate) {
			continue
		}
		isExpired := a.ExpireDate != nil && today.After(*a.ExpireDate)
		level := slog.LevelWarn
		status := "STALE"
		if isExpired {
			level = slog.LevelError
			status = "EXPIRED"
			expired = true
		}
		log.Log(
			context.Background(),
			level,
			fmt.Sprintf("%s: %s:%d [%s]", status, a.File, a.Line, a.Declaration),
			slog.String("stale_date", a.WarnDate.Format("02-01-2006")),
			slog.String("expires", formatDate(a.ExpireDate)),
			slog.String("todo", a.Message),
		)
	}
	return expired
}

func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("02-01-2006")
}
