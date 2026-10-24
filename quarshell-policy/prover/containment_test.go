package prover

import (
	"strings"
	"testing"

	"github.com/KLORQ/quarshell-policy"
)

func policyFrom(t *testing.T, source string) *policy.Policy {
	t.Helper()
	document, err := policy.Parse(strings.NewReader(source), policy.DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	typed, err := document.Typed()
	if err != nil {
		t.Fatalf("Typed: %v", err)
	}
	return typed
}

func TestContainmentWithin(t *testing.T) {
	allowed := policyFrom(t, `version: 1
filesystem_policy:
  read_only: [/usr]
  read_write: [/workspace]
process:
  run_as_user: sandbox
network_policies:
  api:
    endpoints:
      - host: '**.example.com'
        port: 443
        protocol: rest
        access: read-write
`)
	requested := policyFrom(t, `version: 1
filesystem_policy:
  read_only: [/usr/bin]
  read_write: [/workspace/repo]
process:
  run_as_user: sandbox
network_policies:
  api:
    endpoints:
      - host: api.example.com
        port: 443
        protocol: rest
        access: read-only
`)
	report, err := Containment(Query{Allowed: allowed, Requested: requested})
	if err != nil {
		t.Fatalf("Containment: %v", err)
	}
	if !report.Allows() {
		t.Fatalf("report = %+v", report)
	}
}

func TestContainmentExceeds(t *testing.T) {
	allowed := policyFrom(t, `version: 1
filesystem_policy:
  read_only: [/usr]
  read_write: [/workspace]
process:
  run_as_user: sandbox
network_policies:
  api:
    endpoints:
      - host: '*.example.com'
        port: 443
        protocol: rest
        access: read-only
`)
	cases := map[string]string{
		"extra path": `version: 1
filesystem_policy:
  read_only: [/etc]
`,
		"extra host": `version: 1
network_policies:
  api:
    endpoints:
      - host: evil.example.net
        port: 443
        protocol: rest
        access: read-only
`,
		"extra port": `version: 1
network_policies:
  api:
    endpoints:
      - host: api.example.com
        port: 8443
        protocol: rest
        access: read-only
`,
		"broader wildcard": `version: 1
network_policies:
  api:
    endpoints:
      - host: '**.example.com'
        port: 443
        protocol: rest
        access: read-only
`,
		"process mismatch": `version: 1
process:
  run_as_user: root
`,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			report, err := Containment(Query{Allowed: allowed, Requested: policyFrom(t, source)})
			if err != nil {
				t.Fatalf("Containment: %v", err)
			}
			if report.Relation != Exceeds || report.Allows() {
				t.Fatalf("report = %+v", report)
			}
		})
	}
}

func TestContainmentInconclusiveForRules(t *testing.T) {
	allowed := policyFrom(t, `version: 1
network_policies:
  api:
    endpoints:
      - host: api.example.com
        port: 443
        protocol: rest
        rules:
          - allow:
              method: GET
              path: /v1/*
`)
	requested := policyFrom(t, `version: 1
network_policies:
  api:
    endpoints:
      - host: api.example.com
        port: 443
        protocol: rest
        rules:
          - allow:
              method: GET
              path: /v1/models
`)
	report, err := Containment(Query{Allowed: allowed, Requested: requested})
	if err != nil {
		t.Fatalf("Containment: %v", err)
	}
	if report.Relation != Inconclusive || report.Allows() {
		t.Fatalf("report = %+v", report)
	}
}

func TestContainmentFindsDefinitiveCoverAfterInconclusiveCandidate(t *testing.T) {
	allowed := policyFrom(t, `version: 1
network_policies:
  a-complex:
    endpoints:
      - host: api.example.com
        port: 443
        protocol: rest
        rules:
          - allow:
              method: GET
              path: /v1/*
  z-full:
    endpoints:
      - host: api.example.com
        port: 443
        protocol: rest
        access: full
`)
	requested := policyFrom(t, `version: 1
network_policies:
  api:
    endpoints:
      - host: api.example.com
        port: 443
        protocol: rest
        rules:
          - allow:
              method: POST
              path: /v1/write
`)

	report, err := Containment(Query{Allowed: allowed, Requested: requested})
	if err != nil {
		t.Fatalf("Containment: %v", err)
	}
	if !report.Allows() {
		t.Fatalf("definitive full-access grant should prove containment, got %+v", report)
	}
}

func TestContainmentProcessDiagnosticsAreDeterministic(t *testing.T) {
	allowed := policyFrom(t, `version: 1
process:
  run_as_user: sandbox
  run_as_group: sandbox
`)
	requested := policyFrom(t, `version: 1
process:
  run_as_user: root
  run_as_group: root
`)
	wantCounterexamples := []string{"run_as_user", "run_as_group"}
	for i := 0; i < 100; i++ {
		report, err := Containment(Query{Allowed: allowed, Requested: requested})
		if err != nil {
			t.Fatalf("Containment: %v", err)
		}
		if report.Relation != Exceeds || !equalStrings(report.Counterexample, wantCounterexamples) {
			t.Fatalf("Containment() = %+v, want stable counterexamples %v", report, wantCounterexamples)
		}
	}
}

func TestContainmentFailsClosedForUnsupportedNetworkFeatures(t *testing.T) {
	cases := map[string]struct {
		allowed   string
		requested string
	}{
		"middleware": {
			allowed: `version: 1`,
			requested: `version: 1
network_middlewares:
  custom:
    middleware: example
`,
		},
		"allowed middleware": {
			allowed: `version: 1
network_middlewares:
  custom:
    middleware: example
`,
			requested: `version: 1`,
		},
		"policy binaries": {
			allowed: `version: 1
network_policies:
  api:
    binaries:
      - path: /usr/bin/client
    endpoints:
      - host: api.example.com
        port: 443
`,
			requested: `version: 1
network_policies:
  api:
    endpoints:
      - host: api.example.com
        port: 443
`,
		},
		"endpoint allowed IPs": {
			allowed: `version: 1
network_policies:
  api:
    endpoints:
      - host: api.example.com
        port: 443
        allowed_ips: [192.0.2.1]
`,
			requested: `version: 1
network_policies:
  api:
    endpoints:
      - host: api.example.com
        port: 443
`,
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			report, err := Containment(Query{
				Allowed: policyFrom(t, testCase.allowed), Requested: policyFrom(t, testCase.requested),
			})
			if err != nil {
				t.Fatalf("Containment: %v", err)
			}
			if report.Relation != Unsupported || report.Allows() {
				t.Fatalf("unsupported feature must fail closed, got %+v", report)
			}
		})
	}
}

func equalStrings(left, right []string) bool {
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

func TestContainmentRequiresPolicies(t *testing.T) {
	if _, err := Containment(Query{}); err == nil {
		t.Fatalf("nil policies accepted")
	}
}
