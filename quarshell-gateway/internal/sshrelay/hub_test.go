package sshrelay

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRegisterSupervisorUsesConnectionEpochOwnership(t *testing.T) {
	h := NewHub()
	doneFirst := make(chan struct{})
	closed := make(chan struct{}, 1)
	noop := func(string, string) error { return nil }
	unregisterFirst, err := h.RegisterSupervisor("sandbox", "instance", 1, noop, doneFirst, func() { closed <- struct{}{} })
	if err != nil {
		t.Fatalf("RegisterSupervisor first: %v", err)
	}
	if _, err := h.RegisterSupervisor("sandbox", "instance", 1, noop, make(chan struct{}), nil); !errors.Is(err, ErrSupervisorSessionOwned) {
		t.Fatalf("replayed epoch error=%v, want ownership error", err)
	}
	if _, err := h.RegisterSupervisor("sandbox", "other-instance", 2, noop, make(chan struct{}), nil); !errors.Is(err, ErrSupervisorSessionOwned) {
		t.Fatalf("different live instance error=%v, want ownership error", err)
	}
	unregisterSecond, err := h.RegisterSupervisor("sandbox", "instance", 2, noop, make(chan struct{}), nil)
	if err != nil {
		t.Fatalf("higher connection epoch rejected: %v", err)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("replaced supervisor session was not closed")
	}
	if got, ok := h.SupervisorInstance("sandbox"); !ok || got != "instance" {
		t.Fatalf("current supervisor=(%q,%t), want instance", got, ok)
	}
	unregisterFirst()
	if got, ok := h.SupervisorInstance("sandbox"); !ok || got != "instance" {
		t.Fatalf("stale unregister removed replacement=(%q,%t)", got, ok)
	}
	unregisterSecond()
}

func TestSupervisorOwnerLeaseSurvivesDisconnectAndExpires(t *testing.T) {
	h := NewHub()
	noop := func(string, string) error { return nil }
	done := make(chan struct{})
	unregister, err := h.RegisterSupervisor("sandbox", "instance", 4, noop, done, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !h.RenewSupervisorOwner("sandbox", "instance", 4) {
		t.Fatal("current supervisor could not renew owner lease")
	}
	unregister()
	if _, err := h.RegisterSupervisor("sandbox", "replacement", 1, noop, make(chan struct{}), nil); !errors.Is(err, ErrSupervisorSessionOwned) {
		t.Fatalf("different instance claimed a fresh lease: %v", err)
	}
	h.mu.Lock()
	owner := h.owners["sandbox"]
	owner.expiresAt = time.Now().Add(-time.Second)
	h.owners["sandbox"] = owner
	h.mu.Unlock()
	if _, err := h.RegisterSupervisor("sandbox", "replacement", 1, noop, make(chan struct{}), nil); err != nil {
		t.Fatalf("different instance could not claim expired lease: %v", err)
	}
}

func TestWaitConnected(t *testing.T) {
	t.Run("already connected", func(t *testing.T) {
		h := NewHub()
		h.sessions["demo"] = &session{}
		if err := h.WaitConnected(context.Background(), "demo", time.Second); err != nil {
			t.Fatalf("WaitConnected: %v", err)
		}
	})

	t.Run("connects while waiting", func(t *testing.T) {
		h := NewHub()
		go func() {
			time.Sleep(30 * time.Millisecond)
			h.mu.Lock()
			h.sessions["demo"] = &session{}
			h.mu.Unlock()
		}()
		if err := h.WaitConnected(context.Background(), "demo", time.Second); err != nil {
			t.Fatalf("WaitConnected: %v", err)
		}
	})

	t.Run("deadline", func(t *testing.T) {
		h := NewHub()
		err := h.WaitConnected(context.Background(), "demo", time.Millisecond)
		if !errors.Is(err, ErrNotConnected) {
			t.Fatalf("WaitConnected error = %v, want %v", err, ErrNotConnected)
		}
	})

	t.Run("caller cancellation", func(t *testing.T) {
		h := NewHub()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := h.WaitConnected(ctx, "demo", time.Second); !errors.Is(err, context.Canceled) {
			t.Fatalf("WaitConnected error = %v, want context.Canceled", err)
		}
	})
}

func TestMetricsSnapshotCountsAggregateStateWithoutIdentity(t *testing.T) {
	h := NewHub()
	h.sessions["sandbox-a"] = &session{}
	h.sessions["sandbox-b"] = &session{}
	h.pending["channel-secret"] = &pending{sandbox: "sandbox-a"}

	got := h.MetricsSnapshot()
	if got.SupervisorSessions != 2 || got.RelayPending != 1 {
		t.Fatalf("MetricsSnapshot() = %+v", got)
	}
	if nilHub := (*Hub)(nil).MetricsSnapshot(); nilHub.RelayPendingCapacity != maxPendingRelays || nilHub.SupervisorSessions != 0 || nilHub.RelayPending != 0 {
		t.Fatalf("nil MetricsSnapshot() = %+v", nilHub)
	}
}

func TestOpenChannelEnforcesReplicaAndSandboxPendingLimits(t *testing.T) {
	t.Run("per sandbox", func(t *testing.T) {
		h := NewHub()
		h.sessions["target"] = &session{}
		for i := 0; i < maxPendingRelaysPerSandbox; i++ {
			h.pending[fmt.Sprintf("pending-%d", i)] = &pending{sandbox: "target"}
		}
		if _, err := h.OpenChannel(context.Background(), "target", "tcp://127.0.0.1:22"); !errors.Is(err, ErrSandboxCapacity) {
			t.Fatalf("OpenChannel error = %v, want ErrSandboxCapacity", err)
		}
		if got := h.MetricsSnapshot().RelayRejectedSandbox; got != 1 {
			t.Fatalf("sandbox capacity rejects = %d, want 1", got)
		}
	})

	t.Run("replica", func(t *testing.T) {
		h := NewHub()
		h.sessions["target"] = &session{}
		for i := 0; i < maxPendingRelays; i++ {
			h.pending[fmt.Sprintf("pending-%d", i)] = &pending{sandbox: fmt.Sprintf("sandbox-%d", i/maxPendingRelaysPerSandbox)}
		}
		if _, err := h.OpenChannel(context.Background(), "target", "tcp://127.0.0.1:22"); !errors.Is(err, ErrReplicaCapacity) {
			t.Fatalf("OpenChannel error = %v, want ErrReplicaCapacity", err)
		}
		if got := h.MetricsSnapshot().RelayRejectedReplica; got != 1 {
			t.Fatalf("replica capacity rejects = %d, want 1", got)
		}
	})
}

func TestOpenChannelTimeoutIncrementsExpiredMetric(t *testing.T) {
	h := NewHub()
	h.OpenTimeout = 5 * time.Millisecond
	h.sessions["sandbox"] = &session{done: make(chan struct{}), open: func(string, string) error { return nil }}
	if _, err := h.OpenChannel(context.Background(), "sandbox", "tcp://127.0.0.1:22"); !errors.Is(err, ErrOpenTimeout) {
		t.Fatalf("OpenChannel error = %v, want ErrOpenTimeout", err)
	}
	if got := h.MetricsSnapshot().RelayExpired; got != 1 {
		t.Fatalf("relay expired = %d, want 1", got)
	}
}

func TestOpenChannelClaimRecordsDurationHistogram(t *testing.T) {
	h := NewHub()
	h.sessions["sandbox"] = &session{done: make(chan struct{})}
	h.sessions["sandbox"].open = func(channel, _ string) error {
		stream, err := h.AcceptRelay("sandbox", channel)
		if stream != nil {
			_ = stream.Close()
		}
		return err
	}
	client, err := h.OpenChannel(context.Background(), "sandbox", "tcp://127.0.0.1:22")
	if err != nil {
		t.Fatalf("OpenChannel: %v", err)
	}
	_ = client.Close()
	metrics := h.MetricsSnapshot()
	if metrics.RelayClaimCount != 1 || metrics.RelayClaimSumSeconds <= 0 || metrics.RelayClaimBuckets[len(metrics.RelayClaimBuckets)-1] != 1 {
		t.Fatalf("claim histogram = count %d sum %g buckets %v", metrics.RelayClaimCount, metrics.RelayClaimSumSeconds, metrics.RelayClaimBuckets)
	}
}
