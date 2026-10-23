package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func conformanceCommand(root string, args []string) error {
	manifestPath := filepath.Join(root, "plans/conformance/corpus/manifest.yaml")
	candidateArg, oracleArg := "", ""
	timeout := 60 * time.Second
	for i := 0; i < len(args); i++ {
		if i+1 >= len(args) {
			return fmt.Errorf("%s requires a value", args[i])
		}
		i++
		switch args[i-1] {
		case "--manifest":
			manifestPath = resolvePath(root, args[i])
		case "--candidate-cmd":
			candidateArg = args[i]
		case "--oracle-cmd":
			oracleArg = args[i]
		case "--timeout":
			seconds, err := strconv.ParseFloat(args[i], 64)
			if err != nil || seconds <= 0 {
				return fmt.Errorf("invalid timeout %q", args[i])
			}
			timeout = time.Duration(seconds * float64(time.Second))
		default:
			return fmt.Errorf("unknown conformance argument %q", args[i-1])
		}
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var manifest map[string]any
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return err
	}
	cases, ok := manifest["cases"].([]any)
	if !ok {
		return errors.New("manifest must contain a cases list")
	}
	candidateCmd, err := splitCommand(candidateArg)
	if err != nil {
		return err
	}
	if len(candidateCmd) == 0 {
		candidateCmd, err = splitCommand(stringValue(manifest["candidate"]))
		if err != nil {
			return err
		}
	}
	if len(candidateCmd) == 0 {
		return errors.New("no candidate command")
	}
	oracleCmd, err := splitCommand(oracleArg)
	if err != nil {
		return err
	}
	base := filepath.Dir(manifestPath)
	tmp, err := os.MkdirTemp("", "quarshell-conformance-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	failures := 0
	for _, value := range cases {
		row, ok := value.(map[string]any)
		if !ok {
			fmt.Println("FAIL <unnamed>: case must be a mapping")
			failures++
			continue
		}
		id := valueOr(row["id"], "<unnamed>")
		path, err := materializeConformanceCase(row, base, tmp)
		if err != nil {
			fmt.Printf("FAIL %s: %v\n", id, err)
			failures++
			continue
		}
		expected, ok := row["expect"].(map[string]any)
		if len(oracleCmd) > 0 {
			got, message := runJSONCommand(oracleCmd, path, timeout)
			if message != "" {
				fmt.Printf("FAIL %s: oracle %s\n", id, message)
				failures++
				continue
			}
			expected, ok = got, true
		}
		if !ok {
			fmt.Printf("FAIL %s: no expected result\n", id)
			failures++
			continue
		}
		got, message := runJSONCommand(candidateCmd, path, timeout)
		if message != "" {
			fmt.Printf("FAIL %s: candidate %s\n", id, message)
			failures++
			continue
		}
		errs := compareConformance(got, expected, false)
		if len(errs) > 0 {
			fmt.Printf("FAIL %s: %s\n", id, strings.Join(errs, "; "))
			failures++
		} else {
			fmt.Printf("PASS %s\n", id)
		}
	}
	evalCases, _ := manifest["eval_cases"].([]any)
	if len(evalCases) > 0 {
		evalCmd, err := splitCommand(stringValue(manifest["eval_candidate"]))
		if err != nil {
			return err
		}
		if len(evalCmd) == 0 {
			fmt.Fprintln(os.Stderr, "no eval candidate command")
			failures += len(evalCases)
		} else {
			for _, value := range evalCases {
				row, ok := value.(map[string]any)
				if !ok {
					fmt.Println("FAIL <unnamed>: case must be a mapping")
					failures++
					continue
				}
				id := valueOr(row["id"], "<unnamed>")
				path := filepath.Join(base, stringValue(row["file"]))
				expected, ok := row["expect"].(map[string]any)
				if !ok {
					fmt.Printf("FAIL %s: no expected result\n", id)
					failures++
					continue
				}
				got, message := runJSONCommand(evalCmd, path, timeout)
				if message != "" {
					fmt.Printf("FAIL %s: candidate %s\n", id, message)
					failures++
					continue
				}
				errs := compareConformance(got, expected, true)
				if len(errs) > 0 {
					fmt.Printf("FAIL %s: %s\n", id, strings.Join(errs, "; "))
					failures++
				} else {
					fmt.Printf("PASS %s\n", id)
				}
			}
		}
	}
	total := len(cases) + len(evalCases)
	fmt.Printf("conformance: %d/%d passed\n", total-failures, total)
	if failures > 0 {
		return fmt.Errorf("%d conformance cases failed", failures)
	}
	return nil
}

func splitCommand(raw string) ([]string, error) {
	var out []string
	var word strings.Builder
	quote := rune(0)
	escaped, active := false, false
	for _, r := range raw {
		if escaped {
			word.WriteRune(r)
			escaped = false
			active = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			active = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			active = true
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			active = true
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' {
			if active {
				out = append(out, word.String())
				word.Reset()
				active = false
			}
			continue
		}
		word.WriteRune(r)
		active = true
	}
	if escaped || quote != 0 {
		return nil, errors.New("unterminated quote or escape in command")
	}
	if active {
		out = append(out, word.String())
	}
	return out, nil
}

func materializeConformanceCase(row map[string]any, base, tmp string) (string, error) {
	if file := stringValue(row["file"]); file != "" {
		return filepath.Join(base, file), nil
	}
	generation, ok := row["generate"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("%v: missing file or generate", row["id"])
	}
	prefix, fill, suffix := stringValue(generation["prefix"]), valueOr(generation["fill"], "x"), stringValue(generation["suffix"])
	count := 0
	switch v := generation["count"].(type) {
	case int:
		count = v
	case int64:
		count = int(v)
	case uint64:
		count = int(v)
	case float64:
		count = int(v)
	}
	if len([]rune(fill)) != 1 || count < 0 || count > 100_000_000 {
		return "", fmt.Errorf("%v: invalid generated fill or count", row["id"])
	}
	path := filepath.Join(tmp, stringValue(row["id"])+".yaml")
	contents := prefix + strings.Repeat(fill, count) + suffix
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func runJSONCommand(command []string, path string, timeout time.Duration) (map[string]any, string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	args := append(append([]string(nil), command[1:]...), path)
	cmd := exec.CommandContext(ctx, command[0], args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, "timeout"
		}
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return nil, "cannot execute: " + err.Error()
		}
		_ = err
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Sprintf("invalid JSON output: %q stderr=%q", truncate(stdout.String(), 200), truncate(stderr.String(), 200))
	}
	return result, ""
}

func compareConformance(candidate, expected map[string]any, eval bool) []string {
	var problems []string
	keys := []string{"accepted", "version", "error_code"}
	if eval {
		keys = []string{"filesystem", "network", "process"}
	}
	for _, key := range keys {
		want, found := expected[key]
		if !found || want == nil {
			continue
		}
		got, exists := candidate[key]
		if !eval {
			if !exists || fmt.Sprint(got) != fmt.Sprint(want) {
				problems = append(problems, fmt.Sprintf("%s: got %v, want %v", key, got, want))
			}
			continue
		}
		wantList, wok := want.([]any)
		gotList, gok := got.([]any)
		if !gok {
			problems = append(problems, key+": candidate returned no decision list")
			continue
		}
		if !wok || len(gotList) != len(wantList) {
			problems = append(problems, fmt.Sprintf("%s: got %d decisions, want %d", key, len(gotList), len(wantList)))
			continue
		}
		for i, effect := range wantList {
			decision, _ := gotList[i].(map[string]any)
			value := decision["Effect"]
			if value == nil {
				value = decision["effect"]
			}
			actual, ok := value.(string)
			if !ok || !strings.EqualFold(actual, fmt.Sprint(effect)) {
				problems = append(problems, fmt.Sprintf("%s[%d]: got %v, want %v", key, i, value, effect))
			}
		}
	}
	return problems
}

func truncate(value string, max int) string {
	if len(value) > max {
		return value[:max]
	}
	return value
}
