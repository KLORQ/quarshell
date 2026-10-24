// Command quarshell-tools maintains the independent Go module workspace.
//
// Run it from any directory below the workspace with:
//
//	go run tools/cmd/quarshell-tools/*.go workspace [--check|--modules]
//	go run tools/cmd/quarshell-tools/*.go pins [--check] [--ref origin/main]
//	go run tools/cmd/quarshell-tools/*.go dag [--strict]
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const modulePrefix = "github.com/KLORQ/"

type moduleInfo struct {
	Name string
	Dir  string
	Go   string
	Tool string
}

type requirement struct {
	Name    string
	Version string
}

func main() {
	if len(os.Args) < 2 {
		die("usage: quarshell-tools workspace|pins|dag|driver-config [options]")
	}
	root, err := workspaceRoot()
	if err != nil {
		die("%v", err)
	}
	switch os.Args[1] {
	case "workspace":
		err = workspaceCommand(root, os.Args[2:])
	case "pins":
		err = pinsCommand(root, os.Args[2:])
	case "dag":
		err = dagCommand(root, os.Args[2:])
	case "vm-parity":
		err = vmParityCommand(root, os.Args[2:])
	case "backend-config":
		err = backendConfigCommand(root, os.Args[2:])
	case "gateway-config":
		err = gatewayConfigCommand(root, os.Args[2:])
	case "driver-config":
		err = driverConfigCommand(root, os.Args[2:])
	case "inventory":
		err = inventoryCommand(root, os.Args[2:])
	case "standalone":
		err = standaloneCommand(root, os.Args[2:])
	case "clean-generated":
		err = cleanGeneratedCommand(root, os.Args[2:])
	case "capture-worktrees":
		err = captureWorktreesCommand(root, os.Args[2:])
	case "capture-cli-help":
		err = captureCLIHelpCommand(os.Args[2:])
	case "evidence-check":
		err = evidenceCheckCommand(root, os.Args[2:])
	case "evidence-report":
		err = evidenceReportCommand(root, os.Args[2:])
	case "conformance":
		err = conformanceCommand(root, os.Args[2:])
	case "release-manifest":
		err = releaseManifestCommand(root, os.Args[2:])
	case "release-verify":
		err = releaseVerifyCommand(root, os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		die("%v", err)
	}
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func workspaceRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if fileExists(filepath.Join(dir, "tools", "modules.lock")) && fileExists(filepath.Join(dir, "go.work")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("workspace root with tools/modules.lock and go.work not found")
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func moduleInventory(root string) (map[string]moduleInfo, []requirement, error) {
	paths, err := filepath.Glob(filepath.Join(root, "*", "go.mod"))
	if err != nil {
		return nil, nil, err
	}
	slices.Sort(paths)
	modules := make(map[string]moduleInfo)
	required := make(map[requirement]bool)
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		mod := moduleInfo{Dir: filepath.Base(filepath.Dir(path))}
		inRequire := false
		scanner := bufio.NewScanner(bytes.NewReader(content))
		for scanner.Scan() {
			line := strings.SplitN(scanner.Text(), "//", 2)[0]
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			switch fields[0] {
			case "module":
				if len(fields) == 2 {
					mod.Name = fields[1]
				}
			case "go":
				if len(fields) == 2 {
					mod.Go = fields[1]
				}
			case "toolchain":
				if len(fields) == 2 {
					mod.Tool = strings.TrimPrefix(fields[1], "go")
				}
			case "require":
				if len(fields) == 2 && fields[1] == "(" {
					inRequire = true
				} else if len(fields) >= 3 && strings.HasPrefix(fields[2], "v") {
					required[requirement{fields[1], fields[2]}] = true
				}
			case ")":
				inRequire = false
			default:
				if inRequire && len(fields) >= 2 && strings.HasPrefix(fields[1], "v") {
					required[requirement{fields[0], fields[1]}] = true
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, nil, err
		}
		if mod.Name == "" || mod.Go == "" {
			return nil, nil, fmt.Errorf("invalid module/go directive: %s", path)
		}
		if _, exists := modules[mod.Name]; exists {
			return nil, nil, fmt.Errorf("duplicate module identity: %s", mod.Name)
		}
		modules[mod.Name] = mod
	}
	if len(modules) == 0 {
		return nil, nil, errors.New("no sibling Go modules found")
	}
	var deps []requirement
	for dep := range required {
		deps = append(deps, dep)
	}
	slices.SortFunc(deps, func(a, b requirement) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return strings.Compare(a.Version, b.Version)
	})
	return modules, deps, nil
}

func versionParts(raw string) ([]int, error) {
	var parts []int
	for _, part := range strings.Split(raw, ".") {
		value, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid Go version %q", raw)
		}
		parts = append(parts, value)
	}
	return parts, nil
}

func newerVersion(a, b string) (bool, error) {
	if b == "" {
		return true, nil
	}
	left, err := versionParts(a)
	if err != nil {
		return false, err
	}
	right, err := versionParts(b)
	if err != nil {
		return false, err
	}
	for i := 0; i < len(left) || i < len(right); i++ {
		x, y := 0, 0
		if i < len(left) {
			x = left[i]
		}
		if i < len(right) {
			y = right[i]
		}
		if x != y {
			return x > y, nil
		}
	}
	return false, nil
}

func publishedPins(root string) (map[string]string, error) {
	content, err := os.ReadFile(filepath.Join(root, "tools", "modules.lock"))
	if err != nil {
		return nil, err
	}
	tagPattern := regexp.MustCompile(`^v0\.[0-9]+\.[0-9]+$`)
	pins := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !strings.HasPrefix(fields[0], "quarshell-") || !tagPattern.MatchString(fields[1]) {
			return nil, fmt.Errorf("invalid pre-stable module pin: %s", line)
		}
		if _, duplicate := pins[fields[0]]; duplicate {
			return nil, fmt.Errorf("duplicate module pin: %s", fields[0])
		}
		pins[fields[0]] = fields[1]
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(pins) == 0 {
		return nil, errors.New("tools/modules.lock has no published pre-stable module versions")
	}
	return pins, nil
}

func workspaceCommand(root string, args []string) error {
	flags := flag.NewFlagSet("workspace", flag.ContinueOnError)
	check := flags.Bool("check", false, "fail if go.work needs regeneration")
	list := flags.Bool("modules", false, "print sibling module directory names")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || (*check && *list) {
		return errors.New("usage: workspace [--check|--modules]")
	}
	modules, requirements, err := moduleInventory(root)
	if err != nil {
		return err
	}
	var dirs []string
	for _, mod := range modules {
		dirs = append(dirs, mod.Dir)
	}
	slices.Sort(dirs)
	if *list {
		fmt.Println(strings.Join(dirs, " "))
		return nil
	}
	pins, err := publishedPins(root)
	if err != nil {
		return err
	}
	// Include published versions even when a module is currently reached only
	// through a binary or integration workflow.
	required := make(map[requirement]bool)
	for _, dep := range requirements {
		if _, local := modules[dep.Name]; local {
			required[dep] = true
		}
	}
	for name, version := range pins {
		full := modulePrefix + name
		if _, local := modules[full]; local {
			required[requirement{full, version}] = true
		}
	}
	var replacements []requirement
	for dep := range required {
		replacements = append(replacements, dep)
	}
	slices.SortFunc(replacements, func(a, b requirement) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return strings.Compare(a.Version, b.Version)
	})
	var goVersion, toolchain string
	for _, mod := range modules {
		if greater, err := newerVersion(mod.Go, goVersion); err != nil {
			return err
		} else if greater {
			goVersion = mod.Go
		}
		if mod.Tool != "" {
			if greater, err := newerVersion(mod.Tool, toolchain); err != nil {
				return err
			} else if greater {
				toolchain = mod.Tool
			}
		}
	}
	var output strings.Builder
	fmt.Fprintf(&output, "go %s\n", goVersion)
	if toolchain != "" {
		fmt.Fprintf(&output, "\ntoolchain go%s\n", toolchain)
	}
	output.WriteString("\n// Generated by tools/cmd/quarshell-tools from sibling go.mod files and modules.lock.\n")
	output.WriteString("// Each product module remains an independent Git repository.\nuse (\n")
	for _, dir := range dirs {
		fmt.Fprintf(&output, "\t./%s\n", dir)
	}
	output.WriteString(")\n\nreplace (\n")
	for _, dep := range replacements {
		fmt.Fprintf(&output, "\t%s %s => ./%s\n", dep.Name, dep.Version, modules[dep.Name].Dir)
	}
	output.WriteString(")\n")
	target := filepath.Join(root, "go.work")
	current, err := os.ReadFile(target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if *check {
		if string(current) != output.String() {
			return errors.New("go.work is stale; run go run tools/cmd/quarshell-tools/*.go workspace")
		}
		fmt.Println("go.work: current")
		return nil
	}
	if string(current) != output.String() {
		if err := os.WriteFile(target, []byte(output.String()), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("go.work: %d modules, %d local replacements\n", len(modules), len(replacements))
	return nil
}

var repositoryLine = regexp.MustCompile(`^(\s*)repository:\s*([^\s#]+)`)
var keyLine = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_-]*):`)
var refLine = regexp.MustCompile(`^\s*ref:\s*([^\s#]+)`)

func pinsCommand(root string, args []string) error {
	flags := flag.NewFlagSet("pins", flag.ContinueOnError)
	check := flags.Bool("check", false, "fail on stale workflow checkout refs")
	ref := flags.String("ref", "", "read workflow files from a Git ref (check mode only)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || (*ref != "" && !*check) {
		return errors.New("usage: pins [--check] [--ref origin/main]")
	}
	pins, err := publishedPins(root)
	if err != nil {
		return err
	}
	if *check && *ref == "" && hasGitModuleCheckouts(root, pins) {
		*ref = "origin/main"
	}
	if *ref != "" {
		changed, repositories, err := checkWorkflowRef(root, *ref, pins)
		if err != nil {
			return err
		}
		if changed > 0 {
			return fmt.Errorf("%d workflow checkout(s) on %s do not match tools/modules.lock", changed, *ref)
		}
		fmt.Printf("checked %d default branches against tools/modules.lock: PASS\n", repositories)
		return nil
	}
	paths, err := filepath.Glob(filepath.Join(root, "quarshell-*", ".github", "workflows", "*.y*ml"))
	if err != nil {
		return err
	}
	slices.Sort(paths)
	changed := 0
	for _, path := range paths {
		count, err := pinWorkflow(path, pins, *check)
		if err != nil {
			return err
		}
		changed += count
	}
	if *check && changed > 0 {
		return fmt.Errorf("%d workflow checkout(s) do not match tools/modules.lock", changed)
	}
	if *check {
		fmt.Printf("found %d workflow checkout(s) requiring updates\n", changed)
	} else {
		fmt.Printf("pinned %d workflow checkout(s)\n", changed)
	}
	return nil
}

func hasGitModuleCheckouts(root string, pins map[string]string) bool {
	for name := range pins {
		if _, err := os.Stat(filepath.Join(root, name, ".git")); err != nil {
			return false
		}
	}
	return len(pins) > 0
}

func checkWorkflowRef(root, ref string, pins map[string]string) (int, int, error) {
	modules := make([]string, 0, len(pins))
	for name := range pins {
		modules = append(modules, name)
	}
	slices.Sort(modules)
	changed, repositories := 0, 0
	for _, name := range modules {
		modulePath := filepath.Join(root, name)
		if _, err := os.Stat(filepath.Join(modulePath, ".git")); err != nil {
			return changed, repositories, fmt.Errorf("missing Git checkout for %s; run tools/checkout-modules.sh", name)
		}
		if ref == "origin/main" {
			fetch := exec.Command("git", "-C", modulePath, "fetch", "--quiet", "--depth", "1", "origin", "+refs/heads/main:refs/remotes/origin/main")
			if output, err := fetch.CombinedOutput(); err != nil {
				return changed, repositories, fmt.Errorf("fetch %s main: %w: %s", name, err, strings.TrimSpace(string(output)))
			}
		}
		list := exec.Command("git", "-C", modulePath, "ls-tree", "-r", "--name-only", ref, "--", ".github/workflows")
		output, err := list.CombinedOutput()
		if err != nil {
			return changed, repositories, fmt.Errorf("list %s workflows at %s: %w: %s", name, ref, err, strings.TrimSpace(string(output)))
		}
		var workflowCount int
		for _, name := range strings.Split(string(output), "\n") {
			if name == "" || !(strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
				continue
			}
			show := exec.Command("git", "-C", modulePath, "show", ref+":"+name)
			content, err := show.CombinedOutput()
			if err != nil {
				return changed, repositories, fmt.Errorf("read %s:%s: %w: %s", filepath.Base(modulePath), name, err, strings.TrimSpace(string(content)))
			}
			_, count := pinWorkflowContent(content, pins, true)
			changed += count
			workflowCount++
		}
		repositories++
	}
	return changed, repositories, nil
}

func pinWorkflow(path string, pins map[string]string, check bool) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	updated, changed := pinWorkflowContent(content, pins, check)
	if changed == 0 || check {
		return changed, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return changed, os.WriteFile(path, updated, info.Mode().Perm())
}

func pinWorkflowContent(content []byte, pins map[string]string, check bool) ([]byte, int) {
	lines := strings.SplitAfter(string(content), "\n")
	replace := make(map[int]string)
	insert := make(map[int]string)
	changed := 0
	for i, line := range lines {
		match := repositoryLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		repository := strings.TrimPrefix(match[2], "KLORQ/")
		if match[2] != "KLORQ/"+repository {
			continue
		}
		revision, known := pins[repository]
		if !known {
			continue
		}
		indent := match[1]
		refIndex := -1
		for j := i + 1; j < len(lines); j++ {
			following := lines[j]
			if strings.TrimSpace(following) == "" {
				continue
			}
			followingIndent := len(following) - len(strings.TrimLeft(following, " \t"))
			if followingIndent < len(indent) {
				break
			}
			if followingIndent != len(indent) {
				continue
			}
			if key := keyLine.FindStringSubmatch(following); key != nil && key[1] == "ref" {
				refIndex = j
				break
			}
		}
		if refIndex < 0 {
			changed++
			if !check {
				insert[i] = indent + "ref: " + revision + lineEnding(line)
			}
			continue
		}
		if found := refLine.FindStringSubmatch(lines[refIndex]); found == nil || found[1] != revision {
			changed++
			if !check {
				replace[refIndex] = indent + "ref: " + revision + lineEnding(lines[refIndex])
			}
		}
	}
	if changed == 0 || check {
		return content, changed
	}
	var output strings.Builder
	for i, line := range lines {
		if replacement, found := replace[i]; found {
			line = replacement
		}
		output.WriteString(line)
		if insertion, found := insert[i]; found {
			output.WriteString(insertion)
		}
	}
	return []byte(output.String()), changed
}

func lineEnding(line string) string {
	if strings.HasSuffix(line, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func dagCommand(root string, args []string) error {
	flags := flag.NewFlagSet("dag", flag.ContinueOnError)
	strict := flags.Bool("strict", false, "fail on ADR violations")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: dag [--strict]")
	}
	modules, _, err := moduleInventory(root)
	if err != nil {
		return err
	}
	var names []string
	for name := range modules {
		names = append(names, name)
	}
	slices.Sort(names)
	edges := make(map[string]map[string]bool)
	var violations []string
	for _, name := range names {
		source := modules[name].Dir
		edges[source] = make(map[string]bool)
		command := exec.Command("go", "list", "-f", `{{.ImportPath}}|{{join .Imports " "}}`, "./...")
		command.Dir = filepath.Join(root, source)
		output, err := command.CombinedOutput()
		if err != nil {
			if strings.Contains(string(output), "matched no packages") {
				continue
			}
			return fmt.Errorf("%s: go list failed: %s", source, strings.TrimSpace(string(output)))
		}
		for _, line := range strings.Split(string(output), "\n") {
			pkg, raw, found := strings.Cut(line, "|")
			if !found {
				continue
			}
			for _, imported := range strings.Fields(raw) {
				var target string
				for _, candidate := range names {
					if imported == candidate || strings.HasPrefix(imported, candidate+"/") {
						if len(candidate) > len(target) {
							target = candidate
						}
					}
				}
				if target == "" || modules[target].Dir == source {
					continue
				}
				dest := modules[target].Dir
				edges[source][dest] = true
				if forbiddenImport(source, dest) {
					violations = append(violations, fmt.Sprintf("ADR-0002: %s imports %s via %s", source, dest, pkg))
				}
				if source == "quarshell-gateway" && (dest == "quarshell-driver-vm") && !strings.Contains(pkg, "/cmd/") {
					violations = append(violations, fmt.Sprintf("ADR-0002: gateway library imports %s via %s", dest, pkg))
				}
			}
		}
	}
	var sources []string
	for source := range edges {
		sources = append(sources, source)
	}
	slices.Sort(sources)
	count := 0
	for _, targets := range edges {
		count += len(targets)
	}
	fmt.Printf("module DAG: %d modules, %d edges\n", len(modules), count)
	for _, source := range sources {
		var targets []string
		for target := range edges[source] {
			targets = append(targets, target)
		}
		slices.Sort(targets)
		for _, target := range targets {
			fmt.Printf("  %s -> %s\n", source, target)
		}
	}
	if cycle := findCycle(edges); len(cycle) > 0 {
		return fmt.Errorf("module DAG: FAIL cycle: %s", strings.Join(cycle, " -> "))
	}
	for _, violation := range violations {
		if *strict {
			fmt.Fprintln(os.Stderr, "warning:", violation)
		} else {
			fmt.Println("warning:", violation)
		}
	}
	if *strict && len(violations) > 0 {
		return errors.New("module DAG: FAIL (ADR violations)")
	}
	fmt.Println("module DAG: PASS (cycles: 0)")
	return nil
}

func forbiddenImport(source, dest string) bool {
	if source == "quarshell-proto" {
		return true
	}
	if source == "quarshell-core" && dest == "quarshell-proto" {
		return true
	}
	if source != "quarshell-sdk" {
		return false
	}
	switch dest {
	case "quarshell-gateway", "quarshell-cli", "quarshell-runtime",
		"quarshell-driver", "quarshell-driver-vm", "quarshell-proxy",
		"quarshell-policy":
		return true
	}
	return false
}

func findCycle(edges map[string]map[string]bool) []string {
	color := make(map[string]int)
	var stack, cycle []string
	var visit func(string) bool
	visit = func(source string) bool {
		color[source] = 1
		stack = append(stack, source)
		var targets []string
		for target := range edges[source] {
			targets = append(targets, target)
		}
		slices.Sort(targets)
		for _, target := range targets {
			if color[target] == 0 && visit(target) {
				return true
			}
			if color[target] == 1 {
				start := slices.Index(stack, target)
				cycle = append(append([]string{}, stack[start:]...), target)
				return true
			}
		}
		stack = stack[:len(stack)-1]
		color[source] = 2
		return false
	}
	var sources []string
	for source := range edges {
		sources = append(sources, source)
	}
	slices.Sort(sources)
	for _, source := range sources {
		if color[source] == 0 && visit(source) {
			break
		}
	}
	return cycle
}

func vmParityCommand(root string, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: vm-parity")
	}
	manifestPath := filepath.Join(root, "quarshell-driver-vm", "vm", "testdata", "openshell-v0.1.4-parity.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("VM parity inventory: %w", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return fmt.Errorf("VM parity inventory: %w", err)
	}
	versionBytes, err := os.ReadFile(filepath.Join(root, "tools", "upstream", "UPSTREAM_VERSION"))
	if err != nil {
		return fmt.Errorf("VM parity inventory: %w", err)
	}
	commitBytes, err := os.ReadFile(filepath.Join(root, "tools", "upstream", "UPSTREAM_SHA"))
	if err != nil {
		return fmt.Errorf("VM parity inventory: %w", err)
	}
	version, commit := strings.TrimSpace(string(versionBytes)), strings.TrimSpace(string(commitBytes))
	var problems []string
	if manifest["schema_version"] != float64(1) {
		problems = append(problems, "unsupported or missing schema_version")
	}
	baseline, _ := manifest["baseline"].(map[string]any)
	if asString(baseline["version"]) != version || asString(baseline["commit"]) != commit {
		problems = append(problems, "manifest baseline differs from tools/upstream pin")
	}
	entries, ok := manifest["entries"].([]any)
	if !ok || len(entries) == 0 {
		problems = append(problems, "entries must be a non-empty array")
		entries = nil
	}
	validStatus := map[string]bool{"implemented": true, "partial": true, "open": true, "blocked": true, "unsupported": true}
	idPattern := regexp.MustCompile(`^vm\.[a-z0-9][a-z0-9.-]+$`)
	seen := make(map[string]bool)
	for i, item := range entries {
		entry, ok := item.(map[string]any)
		if !ok {
			problems = append(problems, fmt.Sprintf("entries[%d] must be an object", i))
			continue
		}
		feature := asString(entry["id"])
		if !idPattern.MatchString(feature) {
			problems = append(problems, fmt.Sprintf("entries[%d] has invalid feature id %q", i, feature))
		}
		if seen[feature] {
			problems = append(problems, "duplicate feature id "+feature)
		}
		seen[feature] = true
		if !validStatus[asString(entry["status"])] {
			problems = append(problems, fmt.Sprintf("%s: invalid status %q", feature, asString(entry["status"])))
		}
		evidence, evidenceOK := entry["evidence"].([]any)
		if !evidenceOK {
			problems = append(problems, feature+": evidence must be an array")
		}
		problems = append(problems, checkSourceAnchors(root, feature, asString(entry["upstream"]), filepath.Join("tools", "upstream", "openshell-v0.1.4"), "pinned upstream")...)
		problems = append(problems, checkSourceAnchors(root, feature, asString(entry["quarshell"]), "", "Quarshell")...)
		if evidenceOK {
			for _, item := range evidence {
				ref, ok := item.(string)
				if !ok || strings.TrimSpace(ref) == "" {
					problems = append(problems, feature+": evidence references must be non-empty strings")
					continue
				}
				path, symbol, hasSymbol := strings.Cut(ref, "::")
				path = strings.TrimSpace(path)
				source := filepath.Join(root, path)
				info, err := os.Stat(source)
				if err != nil || !info.Mode().IsRegular() {
					problems = append(problems, fmt.Sprintf("%s: missing evidence source %s", feature, path))
					continue
				}
				if hasSymbol {
					content, err := os.ReadFile(source)
					if err != nil || !hasIdentifier(string(content), strings.TrimSpace(symbol)) {
						problems = append(problems, fmt.Sprintf("%s: evidence symbol %q not found in %s", feature, strings.TrimSpace(symbol), path))
					}
				}
			}
		}
		if asString(entry["status"]) == "implemented" && (!evidenceOK || len(evidence) == 0) {
			problems = append(problems, feature+": implemented status requires evidence")
		}
	}
	if len(problems) != 0 {
		fmt.Fprintln(os.Stderr, "VM parity inventory: FAIL")
		for _, problem := range problems {
			fmt.Fprintln(os.Stderr, "-", problem)
		}
		return errors.New("VM parity inventory validation failed")
	}
	fmt.Printf("VM parity inventory: PASS (%d features, %d unique IDs, OpenShell %s %s)\n", len(entries), len(seen), version, commit)
	return nil
}

func asString(value any) string {
	s, _ := value.(string)
	return s
}

var anchorPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

func checkSourceAnchors(root, feature, references, prefix, label string) []string {
	var problems []string
	for _, ref := range strings.Split(references, ";") {
		ref = strings.TrimSpace(ref)
		path, symbol, hasSymbol := strings.Cut(ref, "::")
		if path == "" {
			continue
		}
		source := filepath.Join(root, prefix, path)
		info, err := os.Stat(source)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: missing %s source %s", feature, label, path))
			continue
		}
		if !info.Mode().IsRegular() || !hasSymbol {
			continue
		}
		content, err := os.ReadFile(source)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: cannot read %s source %s: %v", feature, label, path, err))
			continue
		}
		var missing []string
		for _, word := range anchorPattern.FindAllString(symbol, -1) {
			if !hasIdentifier(string(content), word) {
				missing = append(missing, word)
			}
		}
		if len(missing) != 0 {
			problems = append(problems, fmt.Sprintf("%s: %s anchor(s) %s not found in %s", feature, label, strings.Join(missing, ", "), path))
		}
	}
	return problems
}

func hasIdentifier(source, word string) bool {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)
	return pattern.MatchString(source)
}
