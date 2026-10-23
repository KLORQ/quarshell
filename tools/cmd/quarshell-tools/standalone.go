package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var standaloneExcluded = map[string]bool{
	".git": true, ".worktrees": true, "node_modules": true, "target": true,
	".idea": true, ".gode": true, "vendor": true, "dist": true,
	".e2e-tmp": true, "__pycache__": true,
}

func standaloneCommand(root string, args []string) error {
	flags := flag.NewFlagSet("standalone", flag.ContinueOnError)
	module := flags.String("module", "", "build only one module directory")
	strict := flags.Bool("strict", false, "fail when any module build fails")
	timeoutSeconds := flags.Float64("timeout", 180, "per-module timeout in seconds")
	keepProxy := flags.Bool("keep-proxy", false, "keep generated module proxy")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *timeoutSeconds <= 0 {
		return errors.New("usage: standalone [--module DIR] [--strict] [--timeout SECONDS] [--keep-proxy]")
	}
	modules, requirements, err := moduleInventory(root)
	if err != nil {
		return err
	}
	versions := make(map[string]map[string]bool)
	for name := range modules {
		versions[name] = map[string]bool{"v0.0.0": true}
	}
	for _, requirement := range requirements {
		if _, local := modules[requirement.Name]; local {
			versions[requirement.Name][requirement.Version] = true
		}
	}
	proxy, err := os.MkdirTemp("", "quarshell-module-proxy-")
	if err != nil {
		return err
	}
	if !*keepProxy {
		defer os.RemoveAll(proxy)
	}
	var names []string
	for name := range modules {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		return strings.Compare(modules[a].Dir, modules[b].Dir)
	})
	for _, name := range names {
		if err := writeModuleProxy(root, modules[name], versions[name], proxy); err != nil {
			return err
		}
	}
	fmt.Printf("local module proxy: %s\n", proxy)
	failures := 0
	matched := *module == ""
	for _, name := range names {
		info := modules[name]
		if *module != "" && info.Dir != *module {
			continue
		}
		matched = true
		errorLine := buildStandalone(root, info, proxy, time.Duration(*timeoutSeconds*float64(time.Second)))
		if errorLine == "" {
			fmt.Printf("PASS %s\n", info.Dir)
		} else {
			failures++
			fmt.Printf("FAIL %s: %s\n", info.Dir, errorLine)
		}
	}
	if !matched {
		return fmt.Errorf("unknown module directory: %s", *module)
	}
	if failures != 0 {
		fmt.Fprintf(os.Stderr, "standalone build: %d module(s) failed\n", failures)
		if *strict {
			return errors.New("standalone build failed")
		}
		return nil
	}
	fmt.Println("standalone build: all modules passed")
	return nil
}

func escapeModulePath(module string) string {
	var out strings.Builder
	for _, character := range module {
		if character >= 'A' && character <= 'Z' {
			out.WriteByte('!')
			out.WriteRune(character + ('a' - 'A'))
		} else {
			out.WriteRune(character)
		}
	}
	return out.String()
}

func writeModuleProxy(root string, module moduleInfo, versions map[string]bool, proxy string) error {
	source := filepath.Join(root, module.Dir)
	versionsDir := filepath.Join(proxy, filepath.FromSlash(escapeModulePath(module.Name)), "@v")
	if err := os.MkdirAll(versionsDir, 0o755); err != nil {
		return err
	}
	goMod, err := os.ReadFile(filepath.Join(source, "go.mod"))
	if err != nil {
		return err
	}
	var ordered []string
	for version := range versions {
		ordered = append(ordered, version)
	}
	slices.Sort(ordered)
	if err := os.WriteFile(filepath.Join(versionsDir, "list"), []byte(strings.Join(ordered, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	for _, version := range ordered {
		base := filepath.Join(versionsDir, version)
		if err := os.WriteFile(base+".mod", goMod, 0o644); err != nil {
			return err
		}
		info, _ := json.Marshal(map[string]string{"Version": version, "Time": "2026-01-01T00:00:00Z"})
		if err := os.WriteFile(base+".info", info, 0o644); err != nil {
			return err
		}
		if err := zipModuleSource(source, module.Name, version, base+".zip"); err != nil {
			return err
		}
	}
	return nil
}

func walkModuleFiles(source string, visit func(path, relative string, info fs.FileInfo) error) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if standaloneExcluded[entry.Name()] {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			if fileExists(filepath.Join(path, "go.mod")) {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return visit(path, relative, info)
	})
}

func zipModuleSource(source, moduleName, version, destination string) (err error) {
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	archive := zip.NewWriter(file)
	defer func() {
		if closeErr := archive.Close(); err == nil {
			err = closeErr
		}
	}()
	return walkModuleFiles(source, func(path, relative string, info fs.FileInfo) error {
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = moduleName + "@" + version + "/" + filepath.ToSlash(relative)
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, sourceFile)
		closeErr := sourceFile.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func copyModuleSource(source, destination string) error {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	return walkModuleFiles(source, func(path, relative string, info fs.FileInfo) error {
		target := filepath.Join(destination, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		inErr := in.Close()
		if copyErr != nil {
			out.Close()
			return copyErr
		}
		if inErr != nil {
			out.Close()
			return inErr
		}
		return out.Close()
	})
}

func buildStandalone(root string, module moduleInfo, proxy string, timeout time.Duration) string {
	temporary, err := os.MkdirTemp("", "quarshell-standalone-")
	if err != nil {
		return err.Error()
	}
	defer os.RemoveAll(temporary)
	destination := filepath.Join(temporary, module.Dir)
	if err := copyModuleSource(filepath.Join(root, module.Dir), destination); err != nil {
		return err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "./...")
	command.Dir = destination
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"GOWORK=off",
		"GOPROXY=file://"+proxy+",https://proxy.golang.org",
		"GOSUMDB=off",
		"GONOSUMDB=*",
		"GOFLAGS=-mod=mod",
		"GOTOOLCHAIN=local",
	)
	command.Env = env
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Sprintf("timed out after %.0fs", timeout.Seconds())
		}
		first, _, _ := strings.Cut(strings.TrimSpace(output.String()), "\n")
		if first != "" {
			return first
		}
		return "go build failed"
	}
	return ""
}
