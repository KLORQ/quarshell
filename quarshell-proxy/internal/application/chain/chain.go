// Package chain runs ordered middleware stages with per-stage authentication,
// timeouts, and bounded failures.
package chain

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
)

// Request is the mutable egress request passed through stages.
type Request struct {
	Method  string
	Host    string
	Path    string
	Headers map[string]string
	Body    []byte
}

// Stage transforms one request.
type Stage interface {
	Name() string
	Order() int
	Handle(context.Context, *Request) error
}

// Authenticator verifies the plugin identity before Handle is called.
type Authenticator interface {
	Authenticate(context.Context, string) error
}

// FailurePolicy selects behavior when a stage fails or cannot authenticate.
type FailurePolicy int

const (
	FailClosed FailurePolicy = iota
	FailOpen
)

// StageConfig binds one stage to its operational controls.
type StageConfig struct {
	Stage         Stage
	Authenticator Authenticator
	Timeout       time.Duration
	Policy        FailurePolicy
}

// Report describes the stages that ran.
type Report struct {
	Applied []string
	Skipped []string
	Failed  []string
}

// Chain is an ordered, bounded middleware chain.
type Chain struct {
	configs     []StageConfig
	MaxFailures int
}

// New validates and sorts stage configs.
func New(configs []StageConfig, maxFailures int) (*Chain, error) {
	names := map[string]struct{}{}
	orders := map[int]struct{}{}
	sorted := append([]StageConfig(nil), configs...)
	for _, config := range sorted {
		if config.Stage == nil || config.Stage.Name() == "" {
			return nil, domain.Invalidf("chain.new", "stage and name are required")
		}
		if _, exists := names[config.Stage.Name()]; exists {
			return nil, domain.Invalidf("chain.new", "duplicate stage name")
		}
		names[config.Stage.Name()] = struct{}{}
		if _, exists := orders[config.Stage.Order()]; exists {
			return nil, domain.Invalidf("chain.new", "duplicate stage order")
		}
		orders[config.Stage.Order()] = struct{}{}
		if config.Timeout < 0 {
			return nil, domain.Invalidf("chain.new", "stage timeout must not be negative")
		}
	}
	sort.SliceStable(sorted, func(left, right int) bool { return sorted[left].Stage.Order() < sorted[right].Stage.Order() })
	return &Chain{configs: sorted, MaxFailures: maxFailures}, nil
}

// Run applies stages in order. Only a complete success returns a request.
func (c *Chain) Run(ctx context.Context, request Request) (Request, Report, error) {
	report := Report{}
	failures := 0
	for _, config := range c.configs {
		if err := ctx.Err(); err != nil {
			return request, report, domain.Wrap(domain.ErrCanceled, "chain.run", "chain canceled")
		}
		if config.Authenticator != nil {
			if err := config.Authenticator.Authenticate(ctx, config.Stage.Name()); err != nil {
				failures++
				report.Failed = append(report.Failed, config.Stage.Name())
				if config.Policy == FailClosed {
					return request, report, domain.Wrap(domain.ErrUnauthorized, "chain.run", "stage authentication failed")
				}
				report.Skipped = append(report.Skipped, config.Stage.Name())
				if c.exceeded(failures) {
					return request, report, domain.New(domain.CodeUnavailable, "chain.run", "stage failure budget exceeded")
				}
				continue
			}
		}
		stageCtx := ctx
		cancel := func() {}
		if config.Timeout > 0 {
			stageCtx, cancel = context.WithTimeout(ctx, config.Timeout)
		}
		err := config.Stage.Handle(stageCtx, &request)
		cancel()
		if err != nil {
			failures++
			report.Failed = append(report.Failed, config.Stage.Name())
			if config.Policy == FailClosed {
				return request, report, fmt.Errorf("chain: stage %s failed", config.Stage.Name())
			}
			report.Skipped = append(report.Skipped, config.Stage.Name())
			if c.exceeded(failures) {
				return request, report, domain.New(domain.CodeUnavailable, "chain.run", "stage failure budget exceeded")
			}
			continue
		}
		report.Applied = append(report.Applied, config.Stage.Name())
	}
	return request, report, nil
}

func (c *Chain) exceeded(failures int) bool {
	return c.MaxFailures > 0 && failures > c.MaxFailures
}
