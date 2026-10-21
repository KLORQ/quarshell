#!/bin/sh
set -eu

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
report="${1:-$root/quarshell-gateway/internal/httpapi/testdata/openshell_compatibility_gaps.json}"
inventory="${2:-$root/quarshell-gateway/internal/httpapi/testdata/openshell_rpc_inventory.json}"
matrix="${3:-$root/quarshell-gateway/internal/httpapi/testdata/openshell_compatibility_matrix.json}"
pin_file="$root/tools/upstream/UPSTREAM_SHA"
version_file="$root/tools/upstream/UPSTREAM_VERSION"
pin="$(tr -d '[:space:]' < "$pin_file")"
version="$(tr -d '[:space:]' < "$version_file")"

command -v jq >/dev/null 2>&1 || {
  echo "compat gaps: jq is required" >&2
  exit 1
}

# This is a fixed baseline, not a claim that the whole OpenShell contract is
# inventoried. Keep it independent of the report so deleting an open item fails.
jq -e --arg pin "$pin" --arg version "$version" '
  [
    "P0-RPC-CONFORMANCE", "P1-STATE-RECOVERY", "P2-UPSTREAM-PROXY",
    "P3-USERNS-MATRIX", "P4-CREDENTIAL-LIFECYCLE", "P6-RELEASE-GATES",
    "P0-CONFIG-CORPUS", "P0-CLI-SDK-CORPUS", "P1-SUPERVISOR-EVENTS",
    "P4-EXTERNAL-CORPUS", "P6-PUBLISHED-STANDALONE", "P4-LIVE-POLICY",
    "P3-PODMAN-HOST-GATEWAY", "P2-HOST-BOUNDARIES",
    "P0-MAIN-PROCESS-RESTART-POLICY", "P0-OPERATOR-CREDENTIAL-EXPORT",
    "P0-V014-PROVIDER-CONTRACT", "P3-VM-PRODUCTION-GATE"
  ] as $baseline |
  .schema_version == 1 and
  .upstream_version == $version and
  .upstream_commit == $pin and
  .support_status == "partial-alpha" and
  (.gaps | type == "array") and
  ([.gaps[].id] | sort) == ($baseline | sort) and
  ([.gaps[] | select(.status == "open" or .status == "blocked")] | length > 0) and
  all(.gaps[];
    (type == "object") and
    (["id", "area", "field_or_method", "backend", "status", "evidence", "owner"] - keys_unsorted) == [] and
    (all([.id, .area, .field_or_method, .backend, .evidence, .owner][]; type == "string" and (length > 0))) and
    (.status | IN("open", "blocked", "verified"))
  )
' "$report" >/dev/null || {
  echo "compat gaps: report is incomplete or invalid against the pinned baseline" >&2
  exit 1
}

jq -e --arg pin "$pin" --arg version "$version" '
  .upstream_version == $version and
  .upstream_commit == $pin and
  ([.services[].name] | length == (unique | length)) and
  all(.services[];
    (.name | type == "string" and length > 0) and
    (.methods | type == "array") and
    ([.methods[].name] | length == (unique | length))
  ) and
  ([.services[] | select(.name == "openshell.v1.OpenShell") | .methods | length] == [83]) and
  ([.services[] | select(.name == "openshell.compute.v1.ComputeDriver") | .methods | length] == [12]) and
  ([.services[] | select(.name == "openshell.credentials.v1.CredentialDriver") | .methods | length] == [5]) and
  ([.services[] | select(.name == "openshell.middleware.v1.SupervisorMiddleware") | .methods | length] == [4]) and
  ([.services[] | select(.name == "openshell.middleware.v1.HttpResponsePreReturn") | .methods | length] == [1]) and
  ([.services[] | select(.name == "openshell.gateway_interceptor.v1.GatewayInterceptor") | .methods | length] == [3])
' "$inventory" >/dev/null || {
  echo "compat gaps: RPC inventory is incomplete or has the wrong upstream pin" >&2
  exit 1
}

jq -e --arg pin "$pin" --arg version "$version" --slurpfile gaps "$report" '
  .schema_version == 1 and
  .upstream_version == $version and
  .upstream_commit == $pin and
  (.evidence_statuses | sort) == (["implemented", "local_test", "target_host_test", "released"] | sort) and
  (.features | type == "array" and length > 0) and
  ([.features[].id] | sort) == ([$gaps[0].gaps[].id] | sort) and
  all(.features[];
    (type == "object") and
    (["id", "feature", "backend", "public_api", "test_refs", "evidence_status", "gate_result", "scope"] - keys_unsorted) == [] and
    (all([.id, .feature, .evidence_status, .gate_result, .scope][]; type == "string" and length > 0)) and
    (.backend | type == "array" and length > 0 and all(.[]; type == "string" and length > 0)) and
    (.public_api | type == "array" and length > 0 and all(.[]; type == "string" and length > 0)) and
    (.test_refs | type == "array" and length > 0 and all(.[]; type == "string" and length > 0)) and
    (.evidence_status | IN("implemented", "local_test", "target_host_test", "released")) and
    (.gate_result | IN("pass", "partial", "fail", "open"))
  )
' "$matrix" >/dev/null || {
  echo "compat gaps: feature matrix is incomplete or does not match the pinned gap register" >&2
  exit 1
}

for path in $(jq -r '.features[].test_refs[]' "$matrix"); do
  if [ ! -f "$root/$path" ]; then
    echo "compat gaps: feature matrix test reference does not exist: $path" >&2
    exit 1
  fi
done

count="$(jq '(.gaps | length)' "$report")"
open="$(jq '[.gaps[] | select(.status == "open" or .status == "blocked")] | length' "$report")"
commit="$(jq -r '.upstream_commit' "$report")"
echo "compat gaps: PASS ($open open or blocked of $count tracked requirements; feature matrix validated, upstream $version $commit)"
