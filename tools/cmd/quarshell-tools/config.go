package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type configMapping struct {
	RustName string
	GoName   string
}

var rustStructPattern = regexp.MustCompile(`(?s)pub struct (\w+)\s*\{(.*?)\n\}`)
var goStructPattern = regexp.MustCompile(`(?s)type (\w+) struct \{(.*?)\n\}`)
var tomlTagPattern = regexp.MustCompile(`toml:"([^"]+)"`)
var backendCasePattern = regexp.MustCompile(`case\s+((?:"[^"]+"\s*,\s*)*"[^"]+")\s*:`)
var quotedValuePattern = regexp.MustCompile(`"([^"]+)"`)

func backendConfigCommand(root string, args []string) error {
	flags := flag.NewFlagSet("backend-config", flag.ContinueOnError)
	strict := flags.Bool("strict", false, "fail on missing upstream keys")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: backend-config [--strict]")
	}
	upstream := filepath.Join(root, "tools", "upstream", "openshell-v0.1.4", "crates")
	backends := []struct {
		Name     string
		RustPath string
		Struct   string
		GoPath   string
	}{
		{"kubernetes", filepath.Join(upstream, "openshell-driver-kubernetes", "src", "config.rs"), "KubernetesComputeConfig", filepath.Join(root, "quarshell-driver", "internal", "kubernetes", "backend_config.go")},
		{"vm", filepath.Join(upstream, "openshell-driver-vm", "src", "driver.rs"), "VmDriverConfig", filepath.Join(root, "quarshell-driver-vm", "vm", "backend_config.go")},
	}
	missingTotal := 0
	for _, backend := range backends {
		source, err := os.ReadFile(backend.RustPath)
		if err != nil {
			return fmt.Errorf("missing pinned source: %s", backend.RustPath)
		}
		upstreamFields, found := rustFields(string(source), backend.Struct)
		if !found {
			return fmt.Errorf("upstream struct not found: %s", backend.Struct)
		}
		goSource, err := os.ReadFile(backend.GoPath)
		if err != nil {
			return fmt.Errorf("missing Go backend config: %s", backend.GoPath)
		}
		goKeys := make(map[string]bool)
		for _, clause := range backendCasePattern.FindAllStringSubmatch(string(goSource), -1) {
			for _, quoted := range quotedValuePattern.FindAllStringSubmatch(clause[1], -1) {
				goKeys[quoted[1]] = true
			}
		}
		missing, extra := setDifference(upstreamFields, goKeys), setDifference(goKeys, upstreamFields)
		missingTotal += len(missing)
		for _, field := range missing {
			output := os.Stdout
			if *strict {
				output = os.Stderr
			}
			fmt.Fprintf(output, "warning: %s: upstream key '%s' is not decoded\n", backend.Name, field)
		}
		for _, field := range extra {
			fmt.Printf("info: %s: Go-only key '%s'\n", backend.Name, field)
		}
		fmt.Printf("%s: %d upstream keys, %d Go keys, %d missing, %d Go-only\n", backend.Name, len(upstreamFields), len(goKeys), len(missing), len(extra))
	}
	if *strict && missingTotal > 0 {
		return fmt.Errorf("%d upstream backend config keys are not decoded", missingTotal)
	}
	return nil
}

func gatewayConfigCommand(root string, args []string) error {
	flags := flag.NewFlagSet("gateway-config", flag.ContinueOnError)
	strict := flags.Bool("strict", false, "fail on extra legacy fields")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: gateway-config [--strict]")
	}
	upstream := filepath.Join(root, "tools", "upstream", "openshell-v0.1.4", "crates")
	rustSources := []string{
		filepath.Join(upstream, "openshell-server", "src", "config_file.rs"),
		filepath.Join(upstream, "openshell-core", "src", "config.rs"),
	}
	rust := make(map[string]map[string]bool)
	for _, path := range rustSources {
		source, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("missing pinned source: %s", path)
		}
		for _, match := range rustStructPattern.FindAllStringSubmatch(string(source), -1) {
			rust[match[1]], _ = rustFields(string(source), match[1])
		}
	}
	schemaPath := filepath.Join(root, "quarshell-gateway", "internal", "gatewayconfig", "schema.go")
	goSource, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	goStructs := make(map[string]map[string]bool)
	for _, match := range goStructPattern.FindAllStringSubmatch(string(goSource), -1) {
		fields := make(map[string]bool)
		for _, line := range strings.Split(match[2], "\n") {
			if tag := tomlTagPattern.FindStringSubmatch(line); tag != nil {
				name := strings.SplitN(tag[1], ",", 2)[0]
				fields[name] = true
			}
		}
		goStructs[match[1]] = fields
	}
	mapping := []configMapping{
		{"ConfigFile", "File"}, {"OpenShellRoot", "Root"}, {"GatewayFileSection", "Gateway"},
		{"GatewayTlsFileConfig", "TLS"}, {"OcsfLogConfig", "OCSFLog"},
		{"OtlpConfig", "OTLP"}, {"SupervisorFileSection", "Supervisor"},
		{"MiddlewareServiceFileConfig", "Middleware"}, {"OidcConfig", "OIDC"},
		{"MtlsAuthConfig", "MTLSAuth"}, {"GatewayAuthConfig", "Auth"},
		{"GatewayInterceptorConfig", "Interceptor"},
		{"GatewayInterceptorBindingOverride", "Binding"}, {"GatewayJwtConfig", "JWT"},
	}
	var missing, extra []string
	for _, pair := range mapping {
		upstreamFields, found := rust[pair.RustName]
		if !found {
			return fmt.Errorf("upstream struct not found: %s", pair.RustName)
		}
		goFields, found := goStructs[pair.GoName]
		if !found {
			missing = append(missing, fmt.Sprintf("%s: Go struct %s is missing", pair.RustName, pair.GoName))
			continue
		}
		for _, field := range setDifference(upstreamFields, goFields) {
			missing = append(missing, fmt.Sprintf("%s->%s: missing field '%s'", pair.RustName, pair.GoName, field))
		}
		for _, field := range setDifference(goFields, upstreamFields) {
			extra = append(extra, fmt.Sprintf("%s->%s: extra legacy field '%s'", pair.RustName, pair.GoName, field))
		}
	}
	if !goStructs["ProfileSource"]["type"] || !goStructs["ProfileSource"]["name"] {
		missing = append(missing, "GatewayProviderProfileSourceConfig: ProfileSource lacks type/name")
	}
	for _, problem := range missing {
		fmt.Fprintln(os.Stderr, "error:", problem)
	}
	for _, problem := range extra {
		output := os.Stdout
		if *strict {
			output = os.Stderr
		}
		fmt.Fprintln(output, "warning:", problem)
	}
	fieldCount := 0
	for _, fields := range rust {
		fieldCount += len(fields)
	}
	fmt.Printf("gateway config schema: %d structs, %d pinned fields, %d missing, %d legacy\n", len(mapping), fieldCount, len(missing), len(extra))
	if len(missing) != 0 || (*strict && len(extra) != 0) {
		return errors.New("gateway config schema validation failed")
	}
	return nil
}

func rustFields(source, name string) (map[string]bool, bool) {
	for _, match := range rustStructPattern.FindAllStringSubmatch(source, -1) {
		if match[1] != name {
			continue
		}
		fields := make(map[string]bool)
		for _, line := range strings.Split(match[2], "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "pub ") {
				continue
			}
			name, _, found := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, "pub ")), ":")
			if found && strings.TrimSpace(name) != "" {
				fields[strings.TrimSpace(name)] = true
			}
		}
		return fields, true
	}
	return nil, false
}

func setDifference(a, b map[string]bool) []string {
	var result []string
	for field := range a {
		if !b[field] {
			result = append(result, field)
		}
	}
	slices.Sort(result)
	return result
}
