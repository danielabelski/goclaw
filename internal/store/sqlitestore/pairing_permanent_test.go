//go:build sqlite || sqliteonly

package sqlitestore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func newTestSQLitePairingStore(t *testing.T) (*SQLitePairingStore, context.Context) {
	t.Helper()

	db, err := OpenDB(filepath.Join(t.TempDir(), "pairing.db"))
	if err != nil {
		t.Fatalf("OpenDB error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema error: %v", err)
	}

	ctx := store.WithTenantID(context.Background(), store.MasterTenantID)
	return NewSQLitePairingStore(db), ctx
}

func pairTestDevice(t *testing.T, s *SQLitePairingStore, ctx context.Context, senderID string) *store.PairedDeviceData {
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

func findPaired(list []store.PairedDeviceData, senderID string) *store.PairedDeviceData {
	for i := range list {
		if list[i].SenderID == senderID {
			return &list[i]
		}
	}
	return nil
}

func TestSQLitePairing_ApproveSetsDefaultTTL(t *testing.T) {
	s, ctx := newTestSQLitePairingStore(t)

	paired := pairTestDevice(t, s, ctx, "u1")
	if paired.ExpiresAt == nil {
		t.Fatal("ApprovePairing returned nil ExpiresAt, want default TTL")
	}

	got := findPaired(s.ListPaired(ctx), "u1")
	if got == nil || got.ExpiresAt == nil {
		t.Fatalf("ListPaired: want u1 with expiry, got %+v", got)
	}
	want := time.Now().Add(pairedDeviceTTL)
	if d := time.UnixMilli(*got.ExpiresAt).Sub(want); d > time.Minute || d < -time.Minute {
		t.Fatalf("expires_at off by %v from now+TTL", d)
	}
}

func TestSQLitePairing_SetPermanentRoundTrip(t *testing.T) {
	s, ctx := newTestSQLitePairingStore(t)
	pairTestDevice(t, s, ctx, "u1")

	if err := s.SetPairingPermanent(ctx, "u1", "telegram", true); err != nil {
		t.Fatalf("SetPairingPermanent(true): %v", err)
	}
	got := findPaired(s.ListPaired(ctx), "u1")
	if got == nil || got.ExpiresAt != nil {
		t.Fatalf("after permanent: want u1 with nil ExpiresAt, got %+v", got)
	}
	if ok, err := s.IsPaired(ctx, "u1", "telegram"); err != nil || !ok {
		t.Fatalf("IsPaired after permanent = %v, %v; want true", ok, err)
	}

	if err := s.SetPairingPermanent(ctx, "u1", "telegram", false); err != nil {
		t.Fatalf("SetPairingPermanent(false): %v", err)
	}
	got = findPaired(s.ListPaired(ctx), "u1")
	if got == nil || got.ExpiresAt == nil {
		t.Fatalf("after reverting: want u1 with expiry, got %+v", got)
	}
}

func TestSQLitePairing_SetPermanentDoesNotReviveExpired(t *testing.T) {
	s, ctx := newTestSQLitePairingStore(t)
	pairTestDevice(t, s, ctx, "u1")

	if _, err := s.db.ExecContext(ctx, "UPDATE paired_devices SET expires_at = ? WHERE sender_id = ?",
		time.Now().Add(-time.Hour).Round(0), "u1"); err != nil {
		t.Fatalf("expire pairing: %v", err)
	}

	if err := s.SetPairingPermanent(ctx, "u1", "telegram", true); err == nil {
		t.Fatal("SetPairingPermanent on expired pairing: want error, got nil")
	}
	if ok, _ := s.IsPaired(ctx, "u1", "telegram"); ok {
		t.Fatal("expired pairing was revived")
	}
}

func TestSQLitePairing_SetPermanentUnknownDevice(t *testing.T) {
	s, ctx := newTestSQLitePairingStore(t)

	if err := s.SetPairingPermanent(ctx, "nobody", "telegram", true); err == nil {
		t.Fatal("want error for unknown device, got nil")
	}
}
