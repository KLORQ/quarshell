package chain

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type testStage struct {
	name  string
	order int
	fn    func(context.Context, *Request) error
}

func (s *testStage) Name() string { return s.name }
func (s *testStage) Order() int   { return s.order }
func (s *testStage) Handle(ctx context.Context, request *Request) error {
	if s.fn == nil {
		return nil
	}
	return s.fn(ctx, request)
}

type testAuth struct{ err error }

func (a testAuth) Authenticate(context.Context, string) error { return a.err }

func TestChainOrdersAndAppliesStages(t *testing.T) {
	order := []string{}
	chain, err := New([]StageConfig{
		{Stage: &testStage{name: "second", order: 20, fn: func(_ context.Context, request *Request) error {
			order = append(order, "second")
			request.Headers["x-second"] = "1"
			return nil
		}}},
		{Stage: &testStage{name: "first", order: 10, fn: func(_ context.Context, request *Request) error {
			order = append(order, "first")
			request.Headers["x-first"] = "1"
			return nil
		}}},
	}, 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	request, report, err := chain.Run(context.Background(), Request{Headers: map[string]string{}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Join(order, ",") != "first,second" {
		t.Fatalf("order = %v", order)
	}
	if request.Headers["x-first"] != "1" || request.Headers["x-second"] != "1" {
		t.Fatalf("request = %+v", request)
	}
	if len(report.Applied) != 2 {
		t.Fatalf("report = %+v", report)
	}
}

func TestChainFailurePolicies(t *testing.T) {
	boom := errors.New("boom")
	closed, _ := New([]StageConfig{{Stage: &testStage{name: "closed", order: 1, fn: func(context.Context, *Request) error { return boom }}}}, 0)
	if _, _, err := closed.Run(context.Background(), Request{}); err == nil {
		t.Fatalf("fail-closed stage did not abort")
	}
	open, _ := New([]StageConfig{
		{Stage: &testStage{name: "open", order: 1, fn: func(context.Context, *Request) error { return boom }}, Policy: FailOpen},
		{Stage: &testStage{name: "after", order: 2}},
	}, 0)
	_, report, err := open.Run(context.Background(), Request{})
	if err != nil || len(report.Skipped) != 1 || len(report.Applied) != 1 {
		t.Fatalf("fail-open = %+v %v", report, err)
	}
}

func TestChainAuthenticationAndBudget(t *testing.T) {
	authClosed, _ := New([]StageConfig{{Stage: &testStage{name: "auth", order: 1}, Authenticator: testAuth{err: errors.New("bad token")}}}, 0)
	if _, _, err := authClosed.Run(context.Background(), Request{}); err == nil {
		t.Fatalf("authentication failure did not abort")
	}
	authOpen, _ := New([]StageConfig{{Stage: &testStage{name: "auth", order: 1}, Authenticator: testAuth{err: errors.New("bad token")}, Policy: FailOpen}}, 0)
	if _, report, err := authOpen.Run(context.Background(), Request{}); err != nil || len(report.Skipped) != 1 {
		t.Fatalf("auth fail-open = %+v %v", report, err)
	}
	budget, _ := New([]StageConfig{
		{Stage: &testStage{name: "one", order: 1, fn: func(context.Context, *Request) error { return errors.New("x") }}, Policy: FailOpen},
		{Stage: &testStage{name: "two", order: 2, fn: func(context.Context, *Request) error { return errors.New("x") }}, Policy: FailOpen},
	}, 1)
	if _, _, err := budget.Run(context.Background(), Request{}); err == nil {
		t.Fatalf("failure budget was not enforced")
	}
}

func TestChainTimeoutAndValidation(t *testing.T) {
	timeoutStage := &testStage{name: "slow", order: 1, fn: func(ctx context.Context, _ *Request) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	chain, _ := New([]StageConfig{{Stage: timeoutStage, Timeout: 10 * time.Millisecond, Policy: FailOpen}}, 0)
	if _, report, err := chain.Run(context.Background(), Request{}); err != nil || len(report.Skipped) != 1 {
		t.Fatalf("timeout = %+v %v", report, err)
	}
	if _, err := New([]StageConfig{
		{Stage: &testStage{name: "a", order: 1}},
		{Stage: &testStage{name: "b", order: 1}},
	}, 0); err == nil {
		t.Fatalf("duplicate order accepted")
	}
	if _, err := New([]StageConfig{
		{Stage: &testStage{name: "a", order: 1}},
		{Stage: &testStage{name: "a", order: 2}},
	}, 0); err == nil {
		t.Fatalf("duplicate name accepted")
	}
}
