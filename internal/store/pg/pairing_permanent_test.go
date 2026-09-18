package pg

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func newTestPGPairingStore(t *testing.T) (*PGPairingStore, context.Context) {
	t.Helper()
	db := hooksTestDB(t)

	tenantID := uuid.Must(uuid.NewV7())
	if _, err := db.Exec(
		`INSERT INTO tenants (id, name, slug, status) VALUES ($1,$2,$3,'active')`,
		tenantID, "pairing-test-"+tenantID.String()[:8], "pt-"+tenantID.String()); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM paired_devices WHERE tenant_id = $1", tenantID)
		db.Exec("DELETE FROM pairing_requests WHERE tenant_id = $1", tenantID)
		db.Exec("DELETE FROM tenants WHERE id = $1", tenantID)
	})

	return NewPGPairingStore(db), store.WithTenantID(context.Background(), tenantID)
}

func pairTestPGDevice(t *testing.T, s *PGPairingStore, ctx context.Context, senderID string) *store.PairedDeviceData {
	t.Helper()
	code, err := s.RequestPairing(ctx, senderID, "telegram", "chat-1", "default", nil)
	if err != nil {
		t.Fatalf("RequestPairing: %v", err)
	}
	paired, err := s.ApprovePairing(ctx, code, "operator")
	if err != nil {
		t.Fatalf("ApprovePairing: %v", err)
	}
	return paired
}

func findPGPaired(list []store.PairedDeviceData, senderID string) *store.PairedDeviceData {
	for i := range list {
		if list[i].SenderID == senderID {
			return &list[i]
		}
	}
	return nil
}

func TestPGPairing_SetPermanentRoundTrip(t *testing.T) {
	s, ctx := newTestPGPairingStore(t)

	paired := pairTestPGDevice(t, s, ctx, "u1")
	if paired.ExpiresAt == nil {
		t.Fatal("ApprovePairing returned nil ExpiresAt, want default TTL")
	}

	if err := s.SetPairingPermanent(ctx, "u1", "telegram", true); err != nil {
		t.Fatalf("SetPairingPermanent(true): %v", err)
	}
	got := findPGPaired(s.ListPaired(ctx), "u1")
	if got == nil || got.ExpiresAt != nil {
		t.Fatalf("after permanent: want u1 with nil ExpiresAt, got %+v", got)
	}
	if ok, err := s.IsPaired(ctx, "u1", "telegram"); err != nil || !ok {
		t.Fatalf("IsPaired after permanent = %v, %v; want true", ok, err)
	}

	if err := s.SetPairingPermanent(ctx, "u1", "telegram", false); err != nil {
		t.Fatalf("SetPairingPermanent(false): %v", err)
	}
	got = findPGPaired(s.ListPaired(ctx), "u1")
	if got == nil || got.ExpiresAt == nil {
		t.Fatalf("after reverting: want u1 with expiry, got %+v", got)
	}
	want := time.Now().Add(pairedDeviceTTL)
	if d := time.UnixMilli(*got.ExpiresAt).Sub(want); d > time.Minute || d < -time.Minute {
		t.Fatalf("expires_at off by %v from now+TTL", d)
	}
}

func TestPGPairing_SetPermanentDoesNotReviveExpired(t *testing.T) {
	s, ctx := newTestPGPairingStore(t)
	pairTestPGDevice(t, s, ctx, "u1")

	if _, err := s.db.ExecContext(ctx, "UPDATE paired_devices SET expires_at = NOW() - INTERVAL '1 hour' WHERE sender_id = $1 AND tenant_id = $2",
		"u1", store.TenantIDFromContext(ctx)); err != nil {
		t.Fatalf("expire pairing: %v", err)
	}

	if err := s.SetPairingPermanent(ctx, "u1", "telegram", true); err == nil {
		t.Fatal("SetPairingPermanent on expired pairing: want error, got nil")
	}
	if ok, _ := s.IsPaired(ctx, "u1", "telegram"); ok {
		t.Fatal("expired pairing was revived")
	}
}
