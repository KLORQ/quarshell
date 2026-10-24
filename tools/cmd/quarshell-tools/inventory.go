package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const inventoryBaseline = "5a7728c304b5528e954e9ae900cf21f84e1a12b1"

type openshellInventory struct {
	Baseline struct {
		Commit       string `json:"commit"`
		CheckoutHead string `json:"checkout_head"`
	} `json:"baseline"`
	Digest          string           `json:"inventory_digest_sha256"`
	Counts          map[string]int   `json:"counts"`
	Features        []map[string]any `json:"features"`
	Coverage        []map[string]any `json:"coverage_items"`
	OrderedFeatures []orderedObject
	OrderedCoverage []orderedObject
	CurrentDAG      struct {
		GoModules map[string]string   `json:"go_modules"`
		Edges     map[string][]string `json:"edges"`
	} `json:"current_dependency_dag"`
	OrderedRoot orderedObject
}

// inventoryCommand validates the pinned source snapshot and projects its
// reviewed feature/coverage records to the JSON-compatible YAML consumed by docs.
func inventoryCommand(root string, args []string) error {
	write := true
	for _, arg := range args {
		switch arg {
		case "--write":
			write = true
		case "--check":
			write = false
		default:
			return fmt.Errorf("inventory: unknown argument %q", arg)
		}
	}
	upstream := filepath.Join(root, "tools/upstream/openshell-v0.1.4")
	manifest := filepath.Join(root, "plans/inventory/openshell-v0.1.4-inventory.json")
	featuresPath := filepath.Join(root, "plans/inventory/features.yaml")
	data, err := os.ReadFile(manifest)
	if err != nil {
		return err
	}
	var inventory openshellInventory
	if err := json.Unmarshal(data, &inventory); err != nil {
		return fmt.Errorf("inventory: decode source manifest: %w", err)
	}
	orderedRoot, err := parseOrderedJSON(data)
	if err != nil {
		return fmt.Errorf("inventory: decode ordered source manifest: %w", err)
	}
	rootObject, ok := orderedRoot.(orderedObject)
	if !ok {
		return fmt.Errorf("inventory: source manifest root must be an object")
	}
	inventory.OrderedRoot = rootObject
	if inventory.OrderedFeatures, err = orderedRows(rootObject.get("features")); err != nil {
		return err
	}
	if inventory.OrderedCoverage, err = orderedRows(rootObject.get("coverage_items")); err != nil {
		return err
	}
	if inventory.Baseline.Commit != inventoryBaseline {
		return fmt.Errorf("inventory: expected baseline %s, found %s", inventoryBaseline, inventory.Baseline.Commit)
	}
	head, err := exec.Command("git", "-C", upstream, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("inventory: read pinned checkout head: %w", err)
	}
	if strings.TrimSpace(string(head)) != inventoryBaseline || inventory.Baseline.CheckoutHead != inventoryBaseline {
		return fmt.Errorf("inventory: pinned checkout changed; expected %s", inventoryBaseline)
	}
	digest, sourceCount, err := openshellSourceDigest(upstream)
	if err != nil {
		return err
	}
	if digest != inventory.Digest {
		return fmt.Errorf("inventory: pinned source digest changed: expected %s, got %s (%d source files)", inventory.Digest, digest, sourceCount)
	}
	if inventory.Counts["source_files"] != sourceCount {
		return fmt.Errorf("inventory: source file count changed: expected %d, got %d", inventory.Counts["source_files"], sourceCount)
	}
	modules, _, err := moduleInventory(root)
	if err != nil {
		return err
	}
	edges, err := inventoryModuleEdges(root, modules)
	if err != nil {
		return err
	}
	if len(modules) != 10 {
		return fmt.Errorf("inventory: workspace has %d Go modules; expected 10", len(modules))
	}
	moduleDirs := map[string]any{}
	for module, info := range modules {
		moduleDirs[module] = info.Dir
	}
	if !sameStringMap(inventory.CurrentDAG.GoModules, moduleDirs) {
		if !write {
			return fmt.Errorf("inventory: module map is stale; run `inventory --write`")
		}
	}
	if inventory.Counts["current_go_modules"] != len(modules) && !write {
		return fmt.Errorf("inventory: module count is stale; run `inventory --write`")
	}
	if write {
		if err := inventory.OrderedRoot.setPath([]string{"counts", "current_go_modules"}, len(modules)); err != nil {
			return err
		}
		if err := inventory.OrderedRoot.setPath([]string{"current_dependency_dag", "go_modules"}, moduleDirs); err != nil {
			return err
		}
		if err := inventory.OrderedRoot.setPath([]string{"current_dependency_dag", "edges"}, edgeValues(edges)); err != nil {
			return err
		}
		if err := inventory.OrderedRoot.setPath([]string{"current_dependency_dag", "cycles"}, []any{}); err != nil {
			return err
		}
	} else {
		if !sameStringLists(inventory.CurrentDAG.Edges, edges) {
			return fmt.Errorf("inventory: module edges are stale; run `inventory --write`")
		}
	}
	seen := map[string]bool{}
	featureIDs := map[string]bool{}
	for _, feature := range inventory.Features {
		id, _ := feature["id"].(string)
		if id == "" || featureIDs[id] {
			return fmt.Errorf("inventory: missing or duplicate feature ID %q", id)
		}
		featureIDs[id] = true
	}
	for _, item := range inventory.Coverage {
		id, _ := item["id"].(string)
		if id == "" || seen[id] {
			return fmt.Errorf("inventory: missing or duplicate coverage ID %q", id)
		}
		seen[id] = true
		for _, key := range []string{"source", "owner", "acceptance", "status", "decision"} {
			if value, _ := item[key].(string); value == "" {
				return fmt.Errorf("inventory: %s has no %s", id, key)
			}
		}
		refs, ok := item["feature_ids"].([]any)
		if !ok || len(refs) == 0 {
			return fmt.Errorf("inventory: %s has no feature mapping", id)
		}
		for _, ref := range refs {
			feature, _ := ref.(string)
			if !featureIDs[feature] {
				return fmt.Errorf("inventory: %s references unknown feature %q", id, feature)
			}
		}
	}
	generated, err := renderFeatureInventoryYAML(inventory)
	if err != nil {
		return err
	}
	current, err := os.ReadFile(featuresPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if write {
		if err := os.WriteFile(featuresPath, generated, 0o644); err != nil {
			return err
		}
		manifestJSON, err := marshalOrderedJSON(inventory.OrderedRoot)
		if err != nil {
			return err
		}
		if err := os.WriteFile(manifest, manifestJSON, 0o644); err != nil {
			return err
		}
		fmt.Printf("OpenShell inventory: refreshed module graph and feature projection (%d features, %d coverage items)\n", len(inventory.Features), len(inventory.Coverage))
		return nil
	}
	if !bytes.Equal(current, generated) {
		return fmt.Errorf("inventory: feature YAML is stale; run `go run tools/cmd/quarshell-tools/*.go inventory --write`")
	}
	fmt.Printf("OpenShell inventory: PASS (%d upstream files, %d features, %d coverage items, %d Go modules)\n", sourceCount, len(inventory.Features), len(inventory.Coverage), len(modules))
	return nil
}

func openshellSourceDigest(root string) (string, int, error) {
	excluded := map[string]bool{".git": true, "target": true, "node_modules": true, ".venv": true, "dist": true, "build": true, "site": true}
	paths := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && excluded[entry.Name()] {
			return filepath.SkipDir
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	sort.Slice(paths, func(i, j int) bool {
		return slices.Compare(strings.Split(paths[i], "/"), strings.Split(paths[j], "/")) < 0
	})
	h := sha256.New()
	for _, rel := range paths {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", 0, err
		}
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(contents)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), len(paths), nil
}

func renderFeatureInventoryYAML(data openshellInventory) ([]byte, error) {
	var b strings.Builder
	b.WriteString("# Generated from the plan's OS-* feature table and pinned OpenShell inventory.\n")
	b.WriteString("# JSON-compatible YAML is projected from the pinned source-linked inventory.\n")
	b.WriteString("schema_version: 1\nbaseline: \"OpenShell v0.1.4\"\nnamespace_owner: \"KLORQ; module prefix github.com/KLORQ/quarshell-*\"\ninventory: \"openshell-v0.1.4-inventory.json\"\nfeatures:\n")
	featureKeys := []string{"id", "scope", "owner", "acceptance", "status", "baseline"}
	for _, feature := range data.OrderedFeatures {
		b.WriteString("  - id: ")
		if err := appendJSONScalar(&b, feature.get("id")); err != nil {
			return nil, err
		}
		b.WriteByte('\n')
		for _, key := range featureKeys[1:] {
			b.WriteString("    " + key + ": ")
			if err := appendJSONScalar(&b, feature.get(key)); err != nil {
				return nil, err
			}
			b.WriteByte('\n')
		}
	}
	b.WriteString("coverage_items:\n")
	coverageKeys := []string{"id", "kind", "name", "details", "source", "evidence", "owner", "feature_ids", "platforms", "acceptance", "status", "decision"}
	for _, item := range data.OrderedCoverage {
		for i, key := range coverageKeys {
			if i == 0 {
				b.WriteString("  - id: ")
			} else {
				b.WriteString("    " + key + ": ")
			}
			if err := appendJSONScalar(&b, item.get(key)); err != nil {
				return nil, err
			}
			b.WriteByte('\n')
		}
	}
	return []byte(b.String()), nil
}

type orderedPair struct {
	key   string
	value any
}
type orderedObject []orderedPair

func (o orderedObject) get(key string) any {
	for _, pair := range o {
		if pair.key == key {
			return pair.value
		}
	}
	return nil
}

func parseOrderedJSON(data []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	return parseOrderedValue(d)
}

func parseOrderedValue(d *json.Decoder) (any, error) {
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		object := orderedObject{}
		for d.More() {
			keyToken, err := d.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("inventory: invalid object key")
			}
			value, err := parseOrderedValue(d)
			if err != nil {
				return nil, err
			}
			object = append(object, orderedPair{key, value})
		}
		_, err := d.Token()
		return object, err
	case json.Delim('['):
		array := []any{}
		for d.More() {
			value, err := parseOrderedValue(d)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		_, err := d.Token()
		return array, err
	default:
		return token, nil
	}
}

func orderedRows(value any) ([]orderedObject, error) {
	array, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("inventory: expected an array in source manifest")
	}
	rows := make([]orderedObject, 0, len(array))
	for _, value := range array {
		row, ok := value.(orderedObject)
		if !ok {
			return nil, fmt.Errorf("inventory: expected an object row")
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func appendJSONScalar(b *strings.Builder, value any) error {
	switch value := value.(type) {
	case orderedObject:
		b.WriteByte('{')
		for i, pair := range value {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := appendScalar(b, pair.key); err != nil {
				return err
			}
			b.WriteString(": ")
			if err := appendJSONScalar(b, pair.value); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, item := range value {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := appendJSONScalar(b, item); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	default:
		return appendScalar(b, value)
	}
	return nil
}

func appendScalar(b *strings.Builder, value any) error {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return err
	}
	b.Write(bytes.TrimSuffix(encoded.Bytes(), []byte("\n")))
	return nil
}

func inventoryModuleEdges(root string, modules map[string]moduleInfo) (map[string][]string, error) {
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make(map[string][]string, len(modules))
	for _, sourceName := range names {
		source := modules[sourceName].Dir
		cmd := exec.Command("go", "list", "-f", `{{.ImportPath}}|{{join .Imports " "}}`, "./...")
		cmd.Dir = filepath.Join(root, source)
		output, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(output), "matched no packages") {
				continue
			}
			return nil, fmt.Errorf("%s: go list failed: %s", source, strings.TrimSpace(string(output)))
		}
		targets := map[string]bool{}
		for _, line := range strings.Split(string(output), "\n") {
			_, raw, found := strings.Cut(line, "|")
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
				if target != "" && modules[target].Dir != source {
					targets[modules[target].Dir] = true
				}
			}
		}
		for target := range targets {
			result[source] = append(result[source], target)
		}
		sort.Strings(result[source])
	}
	return result, nil
}

func edgeValues(edges map[string][]string) orderedObject {
	keys := make([]string, 0, len(edges))
	for key := range edges {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := orderedObject{}
	for _, key := range keys {
		values := make([]any, len(edges[key]))
		for i, value := range edges[key] {
			values[i] = value
		}
		out = append(out, orderedPair{key, values})
	}
	return out
}

func sameStringMap(actual map[string]string, expected map[string]any) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, value := range expected {
		text, ok := value.(string)
		if !ok || actual[key] != text {
			return false
		}
	}
	return true
}

func sameStringLists(actual, expected map[string][]string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, values := range expected {
		other, ok := actual[key]
		if !ok || len(values) != len(other) {
			return false
		}
		for i := range values {
			if values[i] != other[i] {
				return false
			}
		}
	}
	return true
}

func (o *orderedObject) setPath(path []string, value any) error {
	if len(path) == 0 {
		return fmt.Errorf("inventory: cannot set an empty path")
	}
	for i := range *o {
		if (*o)[i].key != path[0] {
			continue
		}
		if len(path) == 1 {
			(*o)[i].value = orderedValue(value)
			return nil
		}
		child, ok := (*o)[i].value.(orderedObject)
		if !ok {
			return fmt.Errorf("inventory: path %q is not an object", path[0])
		}
		if err := child.setPath(path[1:], value); err != nil {
			return err
		}
		(*o)[i].value = child
		return nil
	}
	return fmt.Errorf("inventory: key %q is missing", path[0])
}

func orderedValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		object := orderedObject{}
		for _, key := range keys {
			object = append(object, orderedPair{key, orderedValue(value[key])})
		}
		return object
	case []any:
		array := make([]any, len(value))
		for i, item := range value {
			array[i] = orderedValue(item)
		}
		return array
	default:
		return value
	}
}

func marshalOrderedJSON(value any) ([]byte, error) {
	var b strings.Builder
	if err := writeOrderedJSON(&b, value, 0); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return []byte(b.String()), nil
}

func writeOrderedJSON(b *strings.Builder, value any, depth int) error {
	indent := func(level int) { b.WriteString(strings.Repeat("  ", level)) }
	switch value := value.(type) {
	case orderedObject:
		b.WriteByte('{')
		if len(value) == 0 {
			b.WriteByte('}')
			return nil
		}
		b.WriteByte('\n')
		for i, pair := range value {
			indent(depth + 1)
			if err := appendScalar(b, pair.key); err != nil {
				return err
			}
			b.WriteString(": ")
			if err := writeOrderedJSON(b, pair.value, depth+1); err != nil {
				return err
			}
			if i+1 < len(value) {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		indent(depth)
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		if len(value) == 0 {
			b.WriteByte(']')
			return nil
		}
		b.WriteByte('\n')
		for i, item := range value {
			indent(depth + 1)
			if err := writeOrderedJSON(b, item, depth+1); err != nil {
				return err
			}
			if i+1 < len(value) {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		indent(depth)
		b.WriteByte(']')
	default:
		return appendScalar(b, value)
	}
	return nil
}
