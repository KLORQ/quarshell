package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type driverConfigMatrix struct {
	SchemaVersion   int                 `json:"schema_version"`
	UpstreamVersion string              `json:"upstream_version"`
	UpstreamCommit  string              `json:"upstream_commit"`
	Scope           string              `json:"scope"`
	Fields          []driverConfigField `json:"fields"`
}

type driverConfigField struct {
	Backend         string          `json:"backend"`
	Field           string          `json:"field"`
	UpstreamType    string          `json:"upstream_type"`
	LocalDecoder    string          `json:"local_decoder"`
	LocalBinding    *string         `json:"local_binding"`
	ValueBehavior   json.RawMessage `json:"value_behavior"`
	Status          string          `json:"status"`
	TestRefs        []string        `json:"test_refs"`
	NullValueCorpus string          `json:"null_value_corpus"`
}

type driverConfigBackend struct {
	structName string
	sourcePath string
	parserPath string
}

var rustConfigField = regexp.MustCompile(`^pub\s+([a-z_][a-z0-9_]*)\s*:\s*(.+),$`)
var rustSkip = regexp.MustCompile(`serde\(skip`)
var goCase = regexp.MustCompile(`case\s+(.+):$`)
var goFieldNames = regexp.MustCompile(`"([a-z_][a-z0-9_]*)"`)
var goConfigTarget = regexp.MustCompile(`cfg\.([A-Z][A-Za-z0-9_]*)\s*(?:,\s*err\s*=|=)`)

func driverConfigCommand(root string, args []string) error {
	write := false
	for _, arg := range args {
		if arg == "--write" {
			write = true
		} else {
			return fmt.Errorf("driver-config: unknown argument %q", arg)
		}
	}
	upstream := filepath.Join(root, "tools/upstream/openshell-v0.1.4/crates")
	backends := map[string]driverConfigBackend{
		"docker": {"DockerComputeConfig", filepath.Join(upstream, "openshell-driver-docker/src/lib.rs"), "quarshell-driver/internal/docker/config.go"},
		"podman": {"PodmanComputeConfig", filepath.Join(upstream, "openshell-driver-podman/src/config.rs"), "quarshell-driver/podman/config.go"},
	}
	fields := map[string]string{}
	bindings := map[string]map[string]string{}
	for _, backend := range []string{"docker", "podman"} {
		spec := backends[backend]
		text, err := os.ReadFile(spec.sourcePath)
		if err != nil {
			return err
		}
		parsed, err := parseRustConfigStruct(string(text), spec.structName)
		if err != nil {
			return err
		}
		for name, typ := range parsed {
			if name == "upstream_proxy" {
				core, err := os.ReadFile(filepath.Join(upstream, "openshell-core/src/config.rs"))
				if err != nil {
					return err
				}
				proxy, err := parseRustConfigStruct(string(core), "UpstreamProxyConfig")
				if err != nil {
					return err
				}
				for n, t := range proxy {
					fields[backend+"."+n] = t
				}
				continue
			}
			fields[backend+"."+name] = typ
		}
		parser, err := os.ReadFile(filepath.Join(root, spec.parserPath))
		if err != nil {
			return err
		}
		bindings[backend] = parseGoConfigCases(string(parser), backend == "docker")
	}
	matrixPath := filepath.Join(root, "quarshell-driver/internal/configcompat/testdata/openshell-v0.1.4-driver-config.json")
	var matrix driverConfigMatrix
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &matrix); err != nil {
		return err
	}
	if matrix.SchemaVersion != 2 || matrix.UpstreamVersion != "v0.1.4" {
		return fmt.Errorf("driver config matrix: unexpected schema or upstream version")
	}
	want := make(map[string]driverConfigField, len(matrix.Fields))
	for _, row := range matrix.Fields {
		want[row.Backend+"."+row.Field] = row
	}
	missing, extra := []string{}, []string{}
	for key := range fields {
		if _, ok := want[key]; !ok {
			missing = append(missing, key)
		}
	}
	for key := range want {
		if _, ok := fields[key]; !ok {
			extra = append(extra, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing)+len(extra) != 0 {
		return fmt.Errorf("driver config matrix: stale or incorrect; missing=%v, extra=%v", missing, extra)
	}
	counts := map[string]int{}
	for key, typ := range fields {
		row := want[key]
		if row.UpstreamType != typ {
			return fmt.Errorf("driver config matrix: %s type changed: matrix=%q source=%q", key, row.UpstreamType, typ)
		}
		if row.Backend != "docker" && row.Backend != "podman" {
			return fmt.Errorf("driver config matrix: invalid backend %q", row.Backend)
		}
		if len(row.TestRefs) == 0 {
			return fmt.Errorf("driver config matrix: missing test references for %s", key)
		}
		for _, ref := range row.TestRefs {
			if _, err := os.Stat(filepath.Join(root, ref)); err != nil {
				return fmt.Errorf("driver config matrix: missing test reference %s", ref)
			}
		}
		var behavior struct {
			Status        string   `json:"status"`
			Omitted       string   `json:"omitted"`
			ExplicitEmpty string   `json:"explicit_empty"`
			ExplicitZero  string   `json:"explicit_zero"`
			RuntimeEffect string   `json:"runtime_effect"`
			EvidenceRefs  []string `json:"evidence_refs"`
		}
		if err := json.Unmarshal(row.ValueBehavior, &behavior); err != nil {
			return fmt.Errorf("driver config matrix: incomplete value behavior for %s", key)
		}
		if behavior.Status != "open" && behavior.Status != "partial" && behavior.Status != "verified" {
			return fmt.Errorf("driver config matrix: invalid value behavior status %q", behavior.Status)
		}
		if behavior.Status != "open" && len(behavior.EvidenceRefs) == 0 {
			return fmt.Errorf("driver config matrix: missing value behavior evidence for %s", key)
		}
		for _, ref := range behavior.EvidenceRefs {
			if _, err := os.Stat(filepath.Join(root, ref)); err != nil {
				return fmt.Errorf("driver config matrix: missing value-behavior evidence %s", ref)
			}
		}
		if row.NullValueCorpus == "" {
			return fmt.Errorf("driver config matrix: missing null-value corpus note for %s", key)
		}
		counts[behavior.Status]++
	}
	if write {
		// --write is intentionally conservative: source-derived field/type changes
		// require review of the per-field notes and are never invented here.
		return fmt.Errorf("driver config matrix: --write requires review of the source diff; update reviewed matrix rows explicitly")
	}
	fmt.Printf("driver config matrix: PASS (%d pinned backend fields; value behavior: %d open, %d partial, %d verified)\n", len(fields), counts["open"], counts["partial"], counts["verified"])
	return nil
}

func parseRustConfigStruct(source, name string) (map[string]string, error) {
	marker := "pub struct " + name + " {"
	start := strings.Index(source, marker)
	if start < 0 {
		return nil, fmt.Errorf("driver config: Rust struct %s not found", name)
	}
	end := strings.Index(source[start:], "\n}")
	if end < 0 {
		return nil, fmt.Errorf("driver config: Rust struct %s is unterminated", name)
	}
	lines := strings.Split(source[start+len(marker):start+end], "\n")
	fields := map[string]string{}
	attributes := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#[") {
			attributes += " " + trimmed
			continue
		}
		match := rustConfigField.FindStringSubmatch(trimmed)
		if len(match) == 3 && !rustSkip.MatchString(attributes) {
			fields[match[1]] = strings.TrimSpace(match[2])
		}
		attributes = ""
	}
	return fields, nil
}

func parseGoConfigCases(source string, docker bool) map[string]string {
	start := strings.Index(source, "func ConfigFromMap(")
	if start < 0 {
		return map[string]string{}
	}
	end := strings.Index(source[start+1:], "\nfunc ")
	if end < 0 {
		end = len(source) - start - 1
	}
	lines := strings.Split(source[start:start+1+end], "\n")
	result := map[string]string{}
	for i, line := range lines {
		match := goCase.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 2 {
			continue
		}
		names := goFieldNames.FindAllStringSubmatch(match[1], -1)
		j := i + 1
		for j < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[j]), "case ") && strings.TrimSpace(lines[j]) != "default:" {
			j++
		}
		block := strings.Join(lines[i+1:j], "\n")
		binding := "validated by field-specific decoder"
		if m := goConfigTarget.FindStringSubmatch(block); len(m) == 2 {
			binding = m[1]
		} else if docker && strings.Contains(match[1], "socket_path") {
			binding = "daemon endpoint returned by ConfigFromMap"
		}
		for _, name := range names {
			result[name[1]] = binding
		}
	}
	return result
}
