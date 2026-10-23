package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var releaseTargets = [][2]string{
	{"darwin", "arm64"},
	{"darwin", "amd64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"windows", "amd64"},
}

var releaseBinaries = []string{"cli", "gateway", "prover"}

type releaseManifest struct {
	Artifacts []releaseArtifact `json:"artifacts"`
	Signed    bool              `json:"signed"`
}

type releaseArtifact struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func releaseManifestCommand(root string, args []string) error {
	flags := flag.NewFlagSet("release-manifest", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: release-manifest DIR")
	}
	dir := flags.Arg(0)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	artifacts, err := releaseArtifacts(dir)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(releaseManifest{Artifacts: artifacts, Signed: false}, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), payload, 0o644); err != nil {
		return err
	}
	fmt.Printf("release manifest: %s (%d artifacts)\n", filepath.Join(dir, "manifest.json"), len(artifacts))
	return nil
}

func releaseVerifyCommand(root string, args []string) error {
	flags := flag.NewFlagSet("release-verify", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: release-verify DIR")
	}
	dir := flags.Arg(0)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	expected, err := expectedReleaseFiles()
	if err != nil {
		return err
	}
	actual, err := releaseArtifacts(dir)
	if err != nil {
		return err
	}
	actualNames := make([]string, 0, len(actual))
	for _, artifact := range actual {
		actualNames = append(actualNames, artifact.Name)
	}
	if !sameStrings(actualNames, expected) {
		return fmt.Errorf("release binaries differ: missing=%v, unexpected=%v", missingStrings(expected, actualNames), missingStrings(actualNames, expected))
	}
	manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest releaseManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	if manifest.Signed {
		return errors.New("dry-run manifest must explicitly report unsigned artifacts")
	}
	if len(manifest.Artifacts) != len(actual) {
		return fmt.Errorf("manifest contains %d artifacts; expected %d", len(manifest.Artifacts), len(actual))
	}
	byName := make(map[string]releaseArtifact, len(actual))
	for _, artifact := range actual {
		byName[artifact.Name] = artifact
	}
	for _, entry := range manifest.Artifacts {
		artifact, ok := byName[entry.Name]
		if !ok || artifact.SHA256 != entry.SHA256 || artifact.Size != entry.Size {
			return fmt.Errorf("manifest metadata mismatch: %s", entry.Name)
		}
	}
	if err := verifyChecksums(dir, byName); err != nil {
		return err
	}
	sbom, err := os.ReadFile(filepath.Join(dir, "SBOM.txt"))
	if err != nil {
		return err
	}
	if strings.Count(string(sbom), "### ") != len(expected) {
		return fmt.Errorf("SBOM must contain %d module records", len(expected))
	}
	if err := verifyReleaseArchives(dir); err != nil {
		return err
	}
	fmt.Printf("verified %d release binaries, manifest, checksums, and SBOM\n", len(expected))
	return nil
}

func releaseArtifacts(dir string) ([]releaseArtifact, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "quarshell-*"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	artifacts := make([]releaseArtifact, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Name() == "SBOM.txt" {
			continue
		}
		digest, err := fileSHA256(path)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, releaseArtifact{Name: info.Name(), SHA256: digest, Size: info.Size()})
	}
	return artifacts, nil
}

func expectedReleaseFiles() ([]string, error) {
	expected := make([]string, 0, len(releaseTargets)*len(releaseBinaries))
	for _, target := range releaseTargets {
		osName, arch := target[0], target[1]
		extension := ""
		if osName == "windows" {
			extension = ".exe"
		}
		for _, name := range releaseBinaries {
			expected = append(expected, fmt.Sprintf("quarshell-%s-%s-%s%s", name, osName, arch, extension))
		}
	}
	sort.Strings(expected)
	return expected, nil
}

func verifyChecksums(dir string, artifacts map[string]releaseArtifact) error {
	data, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return fmt.Errorf("invalid SHA256SUMS line %q", line)
		}
		name := strings.TrimPrefix(fields[1], "*")
		artifact, ok := artifacts[name]
		if !ok {
			if strings.HasPrefix(name, "quarshell_") && strings.HasSuffix(name, ".tar.gz") {
				continue
			}
			return fmt.Errorf("SHA256SUMS references unknown artifact %s", name)
		}
		if fields[0] != artifact.SHA256 {
			return fmt.Errorf("SHA256SUMS digest mismatch for %s", name)
		}
		seen[name] = true
	}
	for name := range artifacts {
		if !seen[name] {
			return fmt.Errorf("SHA256SUMS is missing %s", name)
		}
	}
	return nil
}

func verifyReleaseArchives(dir string) error {
	for _, target := range releaseTargets {
		matches, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("quarshell_*_%s_%s.tar.gz", target[0], target[1])))
		if err != nil {
			return err
		}
		if len(matches) != 1 {
			return fmt.Errorf("expected one release archive for %s/%s, found %d", target[0], target[1], len(matches))
		}
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func missingStrings(expected, actual []string) []string {
	present := make(map[string]bool, len(actual))
	for _, value := range actual {
		present[value] = true
	}
	var missing []string
	for _, value := range expected {
		if !present[value] {
			missing = append(missing, value)
		}
	}
	return missing
}
