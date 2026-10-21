// Package design contains executable design examples, not a sandbox implementation.
package design

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"time"
)

var (
	ErrTooLarge    = errors.New("input exceeds budget")
	ErrUnsupported = errors.New("capability unsupported")
	ErrConflict    = errors.New("generation changed")
	ErrExpired     = errors.New("admission deadline exceeded")
)

// ReadBounded enforces a byte budget before parsing. An event-bounded YAML
// parser must additionally enforce depth, node, alias and scalar budgets.
func ReadBounded(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 || limit == math.MaxInt64 {
		return nil, errors.New("invalid input budget")
	}
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, ErrTooLarge
	}
	return b, nil
}

// Field distinguishes omission, explicit null and an explicit zero value.
// Decode into a fresh DTO. YAML decoding needs its own equivalent AST logic.
type Field[T any] struct {
	Present bool
	Null    bool
	Value   T
}

func (f *Field[T]) UnmarshalJSON(data []byte) error {
	*f = Field[T]{Present: true}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		f.Null = true
		return nil
	}
	return json.Unmarshal(data, &f.Value)
}

type SandboxID string
type Generation uint64
type Capability string

type Backend interface {
	ID() string
	Capabilities() map[Capability]bool
}

type Unsupported struct {
	Backend    string
	Capability Capability
}

func (e *Unsupported) Error() string {
	return fmt.Sprintf("backend %s lacks %s", e.Backend, e.Capability)
}
func (e *Unsupported) Unwrap() error { return ErrUnsupported }

// Registration must reject typed-nil implementations and metadata/interface drift.
// T is an optional extension interface, for example Exec or PolicyUpdater.
func Require[T any](b Backend, capability Capability) (T, error) {
	var zero T
	if b == nil {
		return zero, errors.New("nil backend")
	}
	if b.Capabilities()[capability] {
		if extension, ok := any(b).(T); ok {
			return extension, nil
		}
	}
	return zero, &Unsupported{Backend: b.ID(), Capability: capability}
}

type Revision struct {
	Instance         string
	Generation       Generation
	PolicyHash       string
	ProviderRevision uint64
}

type Admission struct {
	Expected Revision
	Deadline time.Time
	Accepted bool
}

// Accept is pure. The caller persists its output using CAS in a transaction.
// Authentication must supply the actual caller instance before this call.
func (a Admission) Accept(now time.Time, authenticatedInstance string, receipt Revision) (Admission, error) {
	if authenticatedInstance == "" || authenticatedInstance != receipt.Instance || receipt != a.Expected {
		return a, ErrConflict
	}
	// A duplicate receipt after acceptance is idempotent, even after deadline.
	if a.Accepted {
		return a, nil
	}
	if a.Deadline.IsZero() || !now.Before(a.Deadline) {
		return a, ErrExpired
	}
	a.Accepted = true
	return a, nil
}

type Event struct {
	Sandbox  SandboxID
	Sequence uint64
	Kind     string
}

// FanIn demonstrates independent EOF handling. Production streams also carry
// typed source errors, durable cursors and explicit terminal conditions.
// emit must honor ctx and bound any network write.
func FanIn(ctx context.Context, events, logs <-chan Event, emit func(context.Context, Event) error) error {
	for events != nil || logs != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if err := emit(ctx, event); err != nil {
				return err
			}
		case event, ok := <-logs:
			if !ok {
				logs = nil
				continue
			}
			if err := emit(ctx, event); err != nil {
				return err
			}
		}
	}
	return nil
}

type Desired struct {
	ID          SandboxID
	Generation  Generation
	ImageDigest string
}

type Operation struct {
	ID         string
	Sandbox    SandboxID
	Generation Generation
}

// Tx is implemented by a real transactional store. Reserve enforces unique
// (workspace, idempotency key); identical retries return the same operation.
// A changed requestHash for the same key must return a conflict.
type Tx interface {
	Reserve(ctx context.Context, workspace, key, requestHash string) (Operation, bool, error)
	PutDesired(ctx context.Context, desired Desired) error
	Enqueue(ctx context.Context, operation Operation) error
}

type UnitOfWork interface {
	Within(context.Context, func(Tx) error) error
}

func Submit(ctx context.Context, db UnitOfWork, workspace, key, requestHash string, desired Desired) (Operation, error) {
	var op Operation
	err := db.Within(ctx, func(tx Tx) error {
		reserved, existed, err := tx.Reserve(ctx, workspace, key, requestHash)
		if err != nil {
			return err
		}
		op = reserved
		if existed {
			return nil
		}
		desired.ID = op.Sandbox
		desired.Generation = op.Generation
		if err := tx.PutDesired(ctx, desired); err != nil {
			return err
		}
		return tx.Enqueue(ctx, op)
	})
	if err != nil {
		return Operation{}, err
	}
	return op, nil
}

// boundedOutput stores at most limit bytes and causes the copy to terminate
// on excess output. A separate stderr instance is discarded, never returned.
type boundedOutput struct {
	buf   bytes.Buffer
	limit int
}

func (w *boundedOutput) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.buf.Len() {
		return 0, ErrTooLarge
	}
	return w.buf.Write(p)
}

// RunCredentialHelper is only for a trusted, fixed executable and arguments.
// A platform process-group/job controller is additionally required if helpers
// may create descendants; CommandContext alone does not kill a process tree.
func RunCredentialHelper(ctx context.Context, executable string, argv, environment []string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, argv...)
	cmd.Env = append([]string{}, environment...)
	cmd.WaitDelay = 250 * time.Millisecond
	stdout := &boundedOutput{limit: 64 << 10}
	stderr := &boundedOutput{limit: 8 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Do not expose stderr, command arguments or credentials in diagnostics.
		return nil, errors.New("credential helper failed")
	}
	value := bytes.TrimSpace(stdout.buf.Bytes())
	if len(value) == 0 {
		return nil, errors.New("credential helper returned empty value")
	}
	return append([]byte(nil), value...), nil
}
