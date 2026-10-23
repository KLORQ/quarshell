package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var evidenceResults = map[string]bool{"pass": true, "fail": true, "blocked": true, "unverified": true}
var evidenceLevels = map[string]bool{"static": true, "unit": true, "integration": true, "target-host": true, "release": true}
var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var artifactSchemePattern = regexp.MustCompile(`^[a-z]+://`)

func evidenceCheckCommand(root string, args []string) error {
	matrix := filepath.Join(root, "plans", "evidence", "matrix.yaml")
	inventoryPath := filepath.Join(root, "plans", "inventory", "openshell-v0.1.4-inventory.json")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--inventory":
			if i+1 >= len(args) {
				return errors.New("--inventory requires a path")
			}
			i++
			inventoryPath = resolvePath(root, args[i])
		default:
			if strings.HasPrefix(args[i], "-") || matrix != filepath.Join(root, "plans", "evidence", "matrix.yaml") {
				return fmt.Errorf("unknown or duplicate evidence-check argument %q", args[i])
			}
			matrix = resolvePath(root, args[i])
		}
	}
	document, err := readYAMLMap(matrix)
	if err != nil {
		return fmt.Errorf("missing or invalid evidence matrix %s: %w", matrix, err)
	}
	rows, ok := document["rows"].([]any)
	if !ok || len(rows) == 0 {
		return errors.New("evidence matrix has no rows")
	}
	featureIDs := make(map[string]bool)
	if inventory, err := os.ReadFile(inventoryPath); err == nil {
		var decoded struct {
			Features []struct {
				ID string `json:"id"`
			} `json:"features"`
		}
		if err := json.Unmarshal(inventory, &decoded); err != nil {
			return fmt.Errorf("decode feature inventory: %w", err)
		}
		for _, feature := range decoded.Features {
			featureIDs[feature.ID] = true
		}
	}
	var problems []string
	ids := make(map[string]bool)
	counts := make(map[string]int)
	requiredFields := []string{"owner", "scenario", "required", "backend", "host_profile", "source_digest", "command", "result", "evidence_level", "artifact"}
	for index, item := range rows {
		label := fmt.Sprintf("row %d", index+1)
		row, ok := item.(map[string]any)
		if !ok {
			problems = append(problems, label+": expected a mapping")
			continue
		}
		rowID := stringValue(row["id"])
		if rowID == "" {
			problems = append(problems, label+": missing id")
			continue
		}
		label = rowID
		if ids[rowID] {
			problems = append(problems, label+": duplicate id")
		}
		ids[rowID] = true
		featureID := stringValue(row["feature_id"])
		if featureID == "" {
			problems = append(problems, label+": missing feature_id")
		} else if len(featureIDs) > 0 && !featureIDs[featureID] {
			problems = append(problems, fmt.Sprintf("%s: unknown feature_id %q", label, featureID))
		}
		for _, field := range requiredFields {
			if _, found := row[field]; !found {
				problems = append(problems, fmt.Sprintf("%s: missing field %q", label, field))
			}
		}
		result := stringValue(row["result"])
		if !evidenceResults[result] {
			problems = append(problems, fmt.Sprintf("%s: result must be one of %s, got %q", label, strings.Join(sortedKeys(evidenceResults), ", "), result))
			continue
		}
		counts[result]++
		if result == "pass" {
			for _, field := range []string{"backend", "host_profile", "command", "artifact", "evidence_level", "last_verified"} {
				if emptyValue(row[field]) {
					problems = append(problems, fmt.Sprintf("%s: pass requires %q", label, field))
				}
			}
			level := stringValue(row["evidence_level"])
			if level != "" && !evidenceLevels[level] {
				problems = append(problems, fmt.Sprintf("%s: unknown evidence_level %q", label, level))
			}
			digest := stringValue(row["source_digest"])
			if digest == "" || !sha256Pattern.MatchString(digest) {
				problems = append(problems, label+": pass requires a 64-hex source_digest")
			}
		}
		artifact := stringValue(row["artifact"])
		if artifact != "" && !artifactSchemePattern.MatchString(artifact) {
			path := artifact
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, path)
			}
			if _, err := os.Stat(path); err != nil {
				problems = append(problems, fmt.Sprintf("%s: artifact does not exist: %s", label, artifact))
			}
		}
	}
	if upstreamSHA := stringValue(document["upstream_sha"]); upstreamSHA != "" && len(featureIDs) > 0 {
		inventory, err := os.ReadFile(inventoryPath)
		if err == nil {
			var decoded struct {
				Baseline struct {
					Commit string `json:"commit"`
				} `json:"baseline"`
			}
			if json.Unmarshal(inventory, &decoded) == nil && decoded.Baseline.Commit != "" && upstreamSHA != decoded.Baseline.Commit {
				problems = append(problems, fmt.Sprintf("upstream_sha %s != inventory baseline %s", upstreamSHA, decoded.Baseline.Commit))
			}
		}
	}
	summary := []string{
		fmt.Sprintf("blocked=%d", counts["blocked"]),
		fmt.Sprintf("fail=%d", counts["fail"]),
		fmt.Sprintf("pass=%d", counts["pass"]),
		fmt.Sprintf("unverified=%d", counts["unverified"]),
	}
	fmt.Printf("evidence rows=%d %s\n", len(rows), strings.Join(summary, ", "))
	if len(problems) > 0 {
		for _, problem := range problems {
			fmt.Fprintln(os.Stderr, "error:", problem)
		}
		return errors.New("evidence matrix validation failed")
	}
	return nil
}

func evidenceReportCommand(root string, args []string) error {
	matrix := filepath.Join(root, "plans", "evidence", "matrix.yaml")
	output := filepath.Join(root, "plans", "evidence", "STATUS.md")
	check := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--check":
			check = true
		case "--output":
			if i+1 >= len(args) {
				return errors.New("--output requires a path")
			}
			i++
			output = resolvePath(root, args[i])
		default:
			if strings.HasPrefix(args[i], "-") || matrix != filepath.Join(root, "plans", "evidence", "matrix.yaml") {
				return fmt.Errorf("unknown or duplicate evidence-report argument %q", args[i])
			}
			matrix = resolvePath(root, args[i])
		}
	}
	document, err := readYAMLMap(matrix)
	if err != nil {
		return fmt.Errorf("missing evidence matrix %s: %w", matrix, err)
	}
	rendered, err := renderEvidenceReport(document)
	if err != nil {
		return err
	}
	if check {
		current, err := os.ReadFile(output)
		if err != nil || string(current) != rendered {
			return fmt.Errorf("evidence report is stale: run go run tools/cmd/quarshell-tools/*.go evidence-report")
		}
		fmt.Printf("evidence report is current: %s\n", relativeToRoot(root, output))
		return nil
	}
	if err := os.WriteFile(output, []byte(rendered), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", relativeToRoot(root, output))
	return nil
}

func readYAMLMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if document == nil {
		document = make(map[string]any)
	}
	return document, nil
}

func renderEvidenceReport(document map[string]any) (string, error) {
	rawRows, _ := document["rows"].([]any)
	counts := make(map[string]int)
	required := make([]map[string]any, 0)
	rows := make([]map[string]any, 0, len(rawRows))
	for _, raw := range rawRows {
		row, ok := raw.(map[string]any)
		if !ok {
			return "", errors.New("evidence report row must be a mapping")
		}
		rows = append(rows, row)
		counts[stringValue(row["result"])]++
		if truthy(row["required"]) {
			required = append(required, row)
		}
	}
	ready := len(required) > 0
	for _, row := range required {
		if stringValue(row["result"]) != "pass" {
			ready = false
		}
	}
	baseline := valueOr(document["baseline"], "unknown")
	upstreamSHA := valueOr(document["upstream_sha"], "unknown")
	var out strings.Builder
	out.WriteString("# Evidence status\n\n")
	out.WriteString("Generated from `matrix.yaml` by `tools/cmd/quarshell-tools evidence-report`.\n\n")
	fmt.Fprintf(&out, "- Baseline: %s\n", baseline)
	fmt.Fprintf(&out, "- Upstream SHA: `%s`\n", upstreamSHA)
	fmt.Fprintf(&out, "- Rows: %d (required %d)\n", len(rows), len(required))
	promotion := "NOT READY"
	if ready {
		promotion = "READY"
	}
	fmt.Fprintf(&out, "- Promotion: %s\n", promotion)
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	countStrings := make([]string, 0, len(keys))
	for _, key := range keys {
		countStrings = append(countStrings, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	fmt.Fprintf(&out, "- Counts: %s\n\n", strings.Join(countStrings, ", "))
	out.WriteString("| ID | Owner | Required | Result | Level | Backend | Host | Artifact |\n")
	out.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, row := range rows {
		requiredText := "no"
		if truthy(row["required"]) {
			requiredText = "yes"
		}
		fmt.Fprintf(&out, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			stringValue(row["id"]), stringValue(row["owner"]), requiredText,
			valueOr(row["result"], "unverified"), valueOr(row["evidence_level"], "-"),
			valueOr(row["backend"], "-"), valueOr(row["host_profile"], "-"),
			valueOr(row["artifact"], "-"))
	}
	return out.String(), nil
}

func resolvePath(root, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(root, value)
}

func relativeToRoot(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return path
	}
	return filepath.ToSlash(relative)
}

func stringValue(value any) string {
	s, _ := value.(string)
	return s
}

func emptyValue(value any) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return v == ""
	case bool:
		return !v
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return false
}

func truthy(value any) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	}
	return true
}

func valueOr(value any, fallback string) string {
	if emptyValue(value) {
		return fallback
	}
	return fmt.Sprint(value)
}

func sortedKeys(values map[string]bool) []string {
	var keys []string
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
