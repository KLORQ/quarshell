package prover

import (
	"reflect"
	"testing"
)

func TestRiskSafePolicy(t *testing.T) {
	report := Analyze(policyFrom(t, `version: 1
filesystem_policy:
  read_only: [/usr]
process:
  run_as_user: sandbox
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
`))
	if report.Score != 0 || len(report.Findings) != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRiskBroadPolicy(t *testing.T) {
	report := Analyze(policyFrom(t, `version: 1
filesystem_policy:
  read_write: [/]
process:
  run_as_user: root
network_policies:
  all:
    endpoints:
      - host: '*'
        port: 443
        protocol: mcp
        access: full
        mcp:
          allow_all_known_mcp_methods: true
        rules:
          - allow:
              method: '*'
              path: '**'
`))
	if report.Score != 100 {
		t.Fatalf("score = %d", report.Score)
	}
	codes := map[string]bool{}
	for _, finding := range report.Findings {
		codes[finding.Code] = true
	}
	for _, want := range []string{"fs.root_write", "process.root", "net.wildcard_host", "net.full_access", "mcp.allow_all", "l7.allow_all"} {
		if !codes[want] {
			t.Fatalf("missing finding %q in %+v", want, report.Findings)
		}
	}
}

func TestRiskNilPolicy(t *testing.T) {
	if report := Analyze(nil); report.Score != 0 || len(report.Findings) != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRiskFindingsAreDeterministicAcrossNetworkPolicyMapOrder(t *testing.T) {
	document := policyFrom(t, `version: 1
network_policies:
  z-last:
    endpoints:
      - host: '*'
        port: 443
        access: full
  a-first:
    endpoints:
      - host: '**'
        port: 8443
        rules:
          - allow:
              method: '*'
              path: '**'
`)
	want := Analyze(document)
	wantCodes := []string{"net.wildcard_host", "l7.allow_all", "net.wildcard_host", "net.full_access"}
	if len(want.Findings) != len(wantCodes) {
		t.Fatalf("unexpected finding count: %+v", want.Findings)
	}
	for i, code := range wantCodes {
		if want.Findings[i].Code != code {
			t.Fatalf("finding %d code = %q, want %q; findings=%+v", i, want.Findings[i].Code, code, want.Findings)
		}
	}
	for i := 0; i < 100; i++ {
		got := Analyze(document)
		if got.Score != want.Score || !reflect.DeepEqual(got.Findings, want.Findings) {
			t.Fatalf("Analyze() result changed between calls:\n first: %+v\n later: %+v", want, got)
		}
	}
}
