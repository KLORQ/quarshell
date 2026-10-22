package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
)

func TestSupervisorSessionLeaseSurvivesGatewayReopenAndFencesReplay(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gateway.db")
	store, err := lifecycle.NewSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	ref := domain.SandboxRef{Workspace: "ws", Sandbox: "sb"}
	if _, _, err := service.Create(ctx, testRequest()); err != nil {
		t.Fatal(err)
	}
	started, _, err := service.Start(ctx, ref, 1, "start-1")
	if err != nil {
		t.Fatal(err)
	}
	claim := func(svc *Service, instance string, epoch uint64) error {
		return svc.ClaimSupervisorSession(ctx, ref, started.RuntimeGenerationOrInitial(), started.CredentialEpochOrInitial(), fmt.Sprintf("session-%s-%d", instance, epoch), instance, epoch)
	}
	if err := claim(service, "instance-1", 7); err != nil {
		t.Fatalf("initial owner claim: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := lifecycle.NewSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := NewService(reopened)
	if err != nil {
		t.Fatal(err)
	}
	if err := claim(restarted, "instance-2", 1); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("different instance claimed a durable live lease: %v", err)
	}
	if err := claim(restarted, "instance-1", 8); err != nil {
		t.Fatalf("same instance higher connection epoch did not reconnect: %v", err)
	}
	if err := reopened.View(ctx, func(tx lifecycle.ReadTx) error {
		record, exists, err := tx.Get(supervisorLeaseKind, refKey(ref))
		if err != nil || !exists {
			t.Fatalf("reopened owner lease exists=%t err=%v", exists, err)
		}
		var lease supervisorLease
		if err := json.Unmarshal(record.Payload, &lease); err != nil {
			t.Fatal(err)
		}
		if lease.SessionID != "session-instance-1-8" || lease.ConnectionEpoch != 8 {
			t.Fatalf("persisted lease = %+v", lease)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := claim(restarted, "instance-1", 8); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("replayed connection epoch was accepted: %v", err)
	}
	if err := restarted.RenewSupervisorSession(ctx, ref, started.RuntimeGenerationOrInitial(), started.CredentialEpochOrInitial(), "instance-1", 8); err != nil {
		t.Fatalf("renew current owner: %v", err)
	}
	if err := restarted.RenewSupervisorSession(ctx, ref, started.RuntimeGenerationOrInitial(), started.CredentialEpochOrInitial(), "instance-1", 7); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale owner heartbeat was accepted: %v", err)
	}
	if _, _, err := restarted.Stop(ctx, ref, 1, "stop-1"); err != nil {
		t.Fatal(err)
	}
	restartedSandbox, _, err := restarted.Start(ctx, ref, 1, "start-2")
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.ClaimSupervisorSession(ctx, ref, restartedSandbox.RuntimeGenerationOrInitial(), restartedSandbox.CredentialEpochOrInitial(), "session-instance-2", "instance-2", 1); err != nil {
		t.Fatalf("new generation could not claim after explicit Stop: %v", err)
	}
	_ = reopened.Close()
}
