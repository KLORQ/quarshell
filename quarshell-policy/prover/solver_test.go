package prover

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KLORQ/quarshell-core/domain"
)

type fakeSolver struct {
	answer Solution
	script string
	err    error
}

func (f *fakeSolver) Solve(_ context.Context, script string) (Solution, error) {
	f.script = script
	return f.answer, f.err
}

func TestCheckPortsContainedFallback(t *testing.T) {
	contained, err := CheckPortsContained(context.Background(), []uint16{443, 8443}, []uint16{443}, nil)
	if err != nil || !contained {
		t.Fatalf("contained = %v %v", contained, err)
	}
	contained, err = CheckPortsContained(context.Background(), []uint16{443}, []uint16{8443}, nil)
	if err != nil || contained {
		t.Fatalf("not contained = %v %v", contained, err)
	}
	if contained, err := CheckPortsContained(context.Background(), nil, nil, nil); err != nil || !contained {
		t.Fatalf("empty = %v %v", contained, err)
	}
}

func TestCheckPortsContainedSolver(t *testing.T) {
	solver := &fakeSolver{answer: Unsat}
	contained, err := CheckPortsContained(context.Background(), []uint16{443}, []uint16{443, 8443}, solver)
	if err != nil || !contained {
		t.Fatalf("unsat = %v %v", contained, err)
	}
	if solver.script == "" || !strings.Contains(solver.script, "(check-sat)") {
		t.Fatalf("script = %q", solver.script)
	}
	solver.answer = Sat
	contained, err = CheckPortsContained(context.Background(), []uint16{443}, []uint16{8443}, solver)
	if err != nil || contained {
		t.Fatalf("sat = %v %v", contained, err)
	}
	solver.answer = Unknown
	if _, err := CheckPortsContained(context.Background(), []uint16{443}, []uint16{8443}, solver); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("unknown = %v", err)
	}
}

func TestSymbolicReportNeverAllowsUnknownSolverResults(t *testing.T) {
	unsupportedErr := domain.Wrap(domain.ErrUnsupported, "prover.z3", "z3 binary is unavailable")
	unavailableErr := domain.New(domain.CodeUnavailable, "prover.symbolic", "solver timed out")
	cases := []struct {
		name      string
		contained bool
		err       error
		relation  Relation
	}{
		{name: "contained", contained: true, relation: Within},
		{name: "outside", contained: false, relation: Exceeds},
		{name: "unsupported solver", err: unsupportedErr, relation: Inconclusive},
		{name: "inconclusive solver", err: unavailableErr, relation: Inconclusive},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			report := SymbolicReport(testCase.contained, testCase.err)
			if report.Relation != testCase.relation {
				t.Fatalf("SymbolicReport() = %+v, want relation %q", report, testCase.relation)
			}
			if report.Allows() != (testCase.relation == Within) {
				t.Fatalf("SymbolicReport().Allows() = %v for relation %q", report.Allows(), report.Relation)
			}
		})
	}
}

func TestMissingZ3IsUnsupportedAndCannotAllow(t *testing.T) {
	solver := &Z3{Binary: filepath.Join(t.TempDir(), "missing-z3")}
	contained, err := CheckPortsContained(context.Background(), []uint16{443}, []uint16{443}, solver)
	if contained || !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("CheckPortsContained() = %v, %v; want unsupported and not contained", contained, err)
	}
	report := SymbolicReport(contained, err)
	if report.Relation != Inconclusive || report.Allows() {
		t.Fatalf("SymbolicReport() = %+v; unavailable solver must fail closed", report)
	}
}

func TestZ3Integration(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 is not installed")
	}
	solver := &Z3{}
	contained, err := CheckPortsContained(context.Background(), []uint16{443, 8443}, []uint16{443}, solver)
	if err != nil || !contained {
		t.Fatalf("contained = %v %v", contained, err)
	}
	contained, err = CheckPortsContained(context.Background(), []uint16{443}, []uint16{8443}, solver)
	if err != nil || contained {
		t.Fatalf("not contained = %v %v", contained, err)
	}
}
