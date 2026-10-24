package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

var generatedDirectories = map[string]bool{
	".pytest_cache": true, "__pycache__": true, "build": true,
	"dist": true, "node_modules": true, "playwright-report": true,
	".release-dry-run": true, "release-dry-run": true,
	"release-packages": true, ".gode": true,
	"site": true, "target": true,
	"test-results": true,
}

var cleanupExcluded = map[string]bool{".git": true, ".venv": true, "venv": true}

type cleanupTarget struct {
	Repo string
	Path string
	Size int64
	Dir  bool
}

func cleanGeneratedCommand(root string, args []string) error {
	flags := flag.NewFlagSet("clean-generated", flag.ContinueOnError)
	apply := flags.Bool("apply", false, "remove listed ignored generated outputs")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: clean-generated [--apply]")
	}
	roots, err := cleanupRoots(root)
	if err != nil {
		return err
	}
	rootSet := make(map[string]bool)
	for _, path := range roots {
		rootSet[path] = true
	}
	var targets []cleanupTarget
	seen := make(map[string]bool)
	for _, repo := range roots {
		err := filepath.WalkDir(repo, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == repo {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if entry.IsDir() && (cleanupExcluded[entry.Name()] || rootSet[path]) {
				return filepath.SkipDir
			}
			generated := false
			if entry.IsDir() {
				generated = generatedDirectories[entry.Name()] || strings.HasSuffix(entry.Name(), ".egg-info")
			} else {
				generated = entry.Name() == "quarshell-gateway" || strings.HasSuffix(entry.Name(), ".tsbuildinfo")
			}
			if !generated || !gitIgnored(repo, path) {
				return nil
			}
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			if seen[resolved] {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			seen[resolved] = true
			size, err := generatedSize(path)
			if err != nil {
				return err
			}
			targets = append(targets, cleanupTarget{Repo: repo, Path: resolved, Size: size, Dir: entry.IsDir()})
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	var reclaim int64
	operation := "DRY RUN"
	if *apply {
		operation = "REMOVE"
	}
	for _, target := range targets {
		relative, err := filepath.Rel(target.Repo, target.Path)
		if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("generated target escapes worktree: %s", target.Path)
		}
		reclaim += target.Size
		fmt.Printf("%s: %s (%.1f MiB)\n", operation, filepath.ToSlash(relative), float64(target.Size)/(1024*1024))
	}
	fmt.Printf("%s: %d ignored generated paths; %.2f GiB\n", operation, len(targets), float64(reclaim)/(1024*1024*1024))
	if !*apply {
		fmt.Println("Run with --apply to remove these ignored generated paths.")
		return nil
	}
	for _, target := range targets {
		if !gitIgnored(target.Repo, target.Path) {
			return fmt.Errorf("generated target is no longer ignored: %s", target.Path)
		}
		info, err := os.Lstat(target.Path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("generated target became a symlink: %s", target.Path)
		}
		if target.Dir && info.IsDir() {
			if err := os.RemoveAll(target.Path); err != nil {
				return err
			}
		} else if info.Mode().IsRegular() {
			if err := os.Remove(target.Path); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("generated target changed type: %s", target.Path)
		}
	}
	fmt.Println("clean-generated: complete")
	return nil
}

func cleanupRoots(root string) ([]string, error) {
	repos := []string{
		"quarshell-cli", "quarshell-core", "quarshell-docs",
		"quarshell-driver", "quarshell-driver-vm", "quarshell-gateway",
		"quarshell-org-profile", "quarshell-proto",
		"quarshell-policy", "quarshell-proxy", "quarshell-python", "quarshell-runtime",
		"quarshell-sdk", "quarshell-ui",
		"quarshell-python-beta-locked-acceptance",
	}
	candidates := append([]string{root}, repos...)
	seen := make(map[string]bool)
	var roots []string
	for _, candidate := range candidates {
		repo := candidate
		if candidate != root {
			repo = filepath.Join(root, candidate)
		}
		if _, err := os.Lstat(filepath.Join(repo, ".git")); err != nil {
			continue
		}
		command := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain")
		output, err := command.Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(output), "\n") {
			if !strings.HasPrefix(line, "worktree ") {
				continue
			}
			path, err := filepath.EvalSymlinks(strings.TrimPrefix(line, "worktree "))
			if err != nil {
				continue
			}
			info, err := os.Stat(path)
			if err != nil || !info.IsDir() || seen[path] {
				continue
			}
			seen[path] = true
			roots = append(roots, path)
		}
	}
	slices.Sort(roots)
	return roots, nil
}

func gitIgnored(repo, path string) bool {
	relative, err := filepath.Rel(repo, path)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	command := exec.Command("git", "-C", repo, "check-ignore", "--quiet", "--", filepath.ToSlash(relative))
	command.Stdout = &bytes.Buffer{}
	command.Stderr = &bytes.Buffer{}
	return command.Run() == nil
}

func generatedSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	return size, err
}
