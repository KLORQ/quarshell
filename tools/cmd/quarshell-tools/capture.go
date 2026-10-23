package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

type worktreeEntry struct {
	IndexStatus    string `json:"index_status"`
	WorktreeStatus string `json:"worktree_status"`
	Path           string `json:"path"`
}

type worktreeRecord struct {
	Repository    string          `json:"repository"`
	Checkout      string          `json:"checkout"`
	IsPrimary     bool            `json:"is_primary"`
	Branch        any             `json:"branch"`
	Detached      bool            `json:"detached"`
	Head          string          `json:"head"`
	StatusEntries int             `json:"status_entries"`
	Entries       []worktreeEntry `json:"entries"`
}

func captureWorktreesCommand(root string, args []string) error {
	if len(args) > 1 {
		return errors.New("usage: capture-worktrees [OUTPUT.json]")
	}
	output := filepath.Join(root, "plans", "inventory", "openshell-current-worktrees.json")
	if len(args) == 1 {
		absolute, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		output = absolute
	}
	repos, err := filepath.Glob(filepath.Join(root, "quarshell-*"))
	if err != nil {
		return err
	}
	slices.Sort(repos)
	var rows []worktreeRecord
	seen := make(map[string]bool)
	for _, repo := range repos {
		info, err := os.Stat(repo)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, err := os.Lstat(filepath.Join(repo, ".git")); err != nil {
			continue
		}
		listing, err := gitOutput(repo, "worktree", "list", "--porcelain")
		if err != nil {
			return err
		}
		for _, block := range strings.Split(listing, "\n\n") {
			fields := make(map[string]string)
			for _, line := range strings.Split(block, "\n") {
				key, value, found := strings.Cut(line, " ")
				if found {
					fields[key] = value
				}
			}
			checkout := fields["worktree"]
			if checkout == "" {
				continue
			}
			checkout, err = filepath.EvalSymlinks(checkout)
			if err != nil || seen[checkout] {
				continue
			}
			info, err := os.Stat(checkout)
			if err != nil || !info.IsDir() {
				continue
			}
			seen[checkout] = true
			status, err := gitOutput(checkout, "status", "--porcelain=v1", "--untracked-files=all")
			if err != nil {
				return err
			}
			var entries []worktreeEntry
			for _, line := range strings.Split(status, "\n") {
				if len(line) < 4 {
					continue
				}
				entries = append(entries, worktreeEntry{
					IndexStatus: string(line[0]), WorktreeStatus: string(line[1]), Path: line[3:],
				})
			}
			if entries == nil {
				entries = []worktreeEntry{}
			}
			head, err := gitOutput(checkout, "rev-parse", "HEAD")
			if err != nil {
				return err
			}
			primary, _ := filepath.EvalSymlinks(repo)
			var branch any
			if value, found := fields["branch"]; found {
				branch = value
			}
			rows = append(rows, worktreeRecord{
				Repository: filepath.Base(repo), Checkout: checkout, IsPrimary: checkout == primary,
				Branch: branch, Detached: strings.Contains(block, "detached"),
				Head: head, StatusEntries: len(entries), Entries: entries,
			})
		}
	}
	if rows == nil {
		rows = []worktreeRecord{}
	}
	payload := struct {
		SchemaVersion int              `json:"schema_version"`
		CapturedBy    string           `json:"captured_by"`
		Purpose       string           `json:"purpose"`
		WorktreeCount int              `json:"worktree_count"`
		Worktrees     []worktreeRecord `json:"worktrees"`
	}{
		SchemaVersion: 1,
		CapturedBy:    "tools/cmd/quarshell-tools/capture.go",
		Purpose:       "preserve linked worktree state at capture time",
		WorktreeCount: len(rows),
		Worktrees:     rows,
	}
	if err := writeIndentedJSON(output, payload); err != nil {
		return err
	}
	relative, err := filepath.Rel(root, output)
	if err != nil || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		relative = output
	}
	fmt.Printf("captured %d worktrees into %s\n", len(rows), relative)
	return nil
}

func gitOutput(directory string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git -C %s %s: %w", directory, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func writeIndentedJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

type cliHelpRecord struct {
	CommandPath []string `json:"command_path"`
	ExitCode    int      `json:"exit_code"`
	Usage       *string  `json:"usage"`
	Children    []string `json:"children"`
	Flags       []string `json:"flags"`
	HelpText    string   `json:"help_text"`
	HelpSHA256  string   `json:"help_sha256"`
}

type cliRejectedRecord struct {
	CommandPath []string `json:"command_path"`
	ExitCode    int      `json:"exit_code"`
	Message     string   `json:"message"`
}

var childCommandPattern = regexp.MustCompile(`^\s{2}([a-z][a-z0-9-]*)(?::\s+|\s{2,})`)
var helpFlagPattern = regexp.MustCompile(`^\s+(?:-[A-Za-z],\s*)?--[a-z0-9][a-z0-9-]*\b`)

func captureCLIHelpCommand(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: capture-cli-help BINARY OUTPUT.json")
	}
	binary, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return err
	}
	versionCommand := exec.Command(binary, "--version")
	versionOutput, err := versionCommand.Output()
	if err != nil {
		return err
	}
	type commandPath []string
	pending := []commandPath{{}, {"rule"}, {"workspace"}}
	seen := make(map[string]bool)
	var nodes []cliHelpRecord
	rejected := []cliRejectedRecord{}
	for len(pending) > 0 {
		path := pending[0]
		pending = pending[1:]
		if len(path) > 10 || len(nodes)+len(pending) > 10000 {
			return errors.New("CLI help tree exceeds bounded capture limits")
		}
		key := strings.Join(path, "\x00")
		if seen[key] {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, binary, append(append([]string{}, path...), "--help")...)
		output, err := command.CombinedOutput()
		timedOut := ctx.Err() == context.DeadlineExceeded
		cancel()
		if timedOut {
			return fmt.Errorf("CLI help timed out at %q", strings.Join(path, " "))
		}
		exitCode := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				return err
			}
			exitCode = exit.ExitCode()
		}
		help := string(output)
		if exitCode != 0 {
			rejected = append(rejected, cliRejectedRecord{CommandPath: path, ExitCode: exitCode, Message: firstNonemptyLine(help)})
			continue
		}
		seen[key] = true
		children := helpCommands(help)
		sum := sha256.Sum256([]byte(help))
		nodes = append(nodes, cliHelpRecord{
			CommandPath: path, ExitCode: exitCode, Usage: helpUsage(help),
			Children: children, Flags: helpFlags(help), HelpText: help,
			HelpSHA256: hex.EncodeToString(sum[:]),
		})
		for _, child := range children {
			pending = append(pending, append(append(commandPath{}, path...), child))
		}
	}
	if err := writeIndentedJSON(args[1], struct {
		Baseline            string              `json:"baseline"`
		Binary              string              `json:"binary"`
		Version             string              `json:"version"`
		ValidCommandPaths   int                 `json:"valid_command_paths"`
		CommandHelp         []cliHelpRecord     `json:"command_help"`
		RejectedHelpEntries []cliRejectedRecord `json:"rejected_help_entries"`
		Source              string              `json:"source"`
	}{
		Baseline: "OpenShell v0.1.4", Binary: binary, Version: strings.TrimSpace(string(versionOutput)),
		ValidCommandPaths: len(nodes), CommandHelp: nodes, RejectedHelpEntries: rejected,
		Source: "runtime `openshell --help` recursively invoked; no product operations executed",
	}); err != nil {
		return err
	}
	fmt.Printf("captured %d command help pages; %d displayed entries are not executable\n", len(nodes), len(rejected))
	return nil
}

func helpCommands(help string) []string {
	inCommands := false
	var result []string
	stop := map[string]bool{"FLAGS": true, "EXAMPLES": true, "ALIAS": true, "LEARN MORE": true, "GATEWAY FLAGS": true, "GLOBAL FLAGS": true}
	for _, line := range strings.Split(help, "\n") {
		heading := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(line), ":"))
		if heading == "COMMANDS" || strings.HasSuffix(heading, "COMMANDS") {
			inCommands = true
			continue
		}
		if stop[heading] {
			inCommands = false
			continue
		}
		if inCommands {
			if match := childCommandPattern.FindStringSubmatch(line); match != nil {
				result = append(result, match[1])
			}
		}
	}
	if result == nil {
		return []string{}
	}
	return result
}

func helpFlags(help string) []string {
	set := make(map[string]bool)
	for _, line := range strings.Split(help, "\n") {
		if !helpFlagPattern.MatchString(line) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		first := strings.TrimRight(fields[0], ",.")
		set[first] = true
		if strings.Contains(fields[0], ",") && len(fields) > 1 {
			set[strings.TrimRight(fields[1], ",.")] = true
		}
	}
	var result []string
	for flag := range set {
		result = append(result, flag)
	}
	slices.Sort(result)
	if result == nil {
		return []string{}
	}
	return result
}

func helpUsage(help string) *string {
	for _, line := range strings.Split(help, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if strings.Contains(lower, "openshell ") && (strings.HasPrefix(lower, "openshell ") || strings.HasPrefix(lower, "usage:")) {
			return &line
		}
	}
	return nil
}

func firstNonemptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
