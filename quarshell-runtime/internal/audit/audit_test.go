package audit

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"
)

var auditLinePattern = regexp.MustCompile(`^\d{4}-\d\d-\d\dT[^ ]+ OCSF [A-Z_]+:[A-Z_]+ \[(INFO|LOW|MED|HIGH|CRIT|FATAL)\] [A-Z_]+ .+$`)

func TestFormatProducesTimestampedOCSFObservation(t *testing.T) {
	line := Format(Event{Class: " NET ", Activity: "OPEN", Severity: "HIGH", Action: "DENIED", Details: "connect", Context: "sandbox=demo"})
	if !auditLinePattern.MatchString(line) {
		t.Fatalf("Format() returned non-OCSF observation: %q", line)
	}
	if !strings.HasSuffix(line, "[sandbox=demo]") {
		t.Fatalf("context was not bracketed: %q", line)
	}
	stamp := strings.SplitN(line, " ", 2)[0]
	if _, err := time.Parse(time.RFC3339Nano, stamp); err != nil {
		t.Fatalf("timestamp %q is not RFC3339Nano: %v", stamp, err)
	}
}

func TestWriterEmitterUsesStableFallbackFields(t *testing.T) {
	var output bytes.Buffer
	WriterEmitter{W: &output}.Emit(Event{})
	line := strings.TrimSpace(output.String())
	if !strings.Contains(line, " OCSF NET:OTHER [INFO] OTHER -") {
		t.Fatalf("empty event did not use safe schema defaults: %q", line)
	}
}
