package methods

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// stubPermanentPairingStore answers SetPairingPermanent with a fixed error.
type stubPermanentPairingStore struct {
	store.PairingStore
	err error
}

func (s *stubPermanentPairingStore) SetPairingPermanent(context.Context, string, string, bool) error {
	return s.err
}

func callPairingUpdate(t *testing.T, storeErr error) protocol.ResponseFrame {
	t.Helper()
	m := NewPairingMethods(&stubPermanentPairingStore{err: storeErr}, bus.New(), nil)
	client, out := gateway.NewCapturingTestClient(permissions.RoleAdmin, store.MasterTenantID, "admin-1", 1)
	raw, _ := json.Marshal(map[string]any{"senderId": "u1", "channel": "telegram", "permanent": true})

	m.handleUpdate(t.Context(), client, &protocol.RequestFrame{ID: "r1", Params: raw})

	var frame protocol.ResponseFrame
	if err := json.Unmarshal(<-out, &frame); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return frame
}

func TestPairingUpdate_ErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		storeErr error
		wantCode string
	}{
		{"not found", fmt.Errorf("%w: telegram/u1", store.ErrPairedDeviceNotFound), protocol.ErrNotFound},
		{"db failure", errors.New("connection reset"), protocol.ErrInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frame := callPairingUpdate(t, tc.storeErr)
			if frame.OK || frame.Error == nil || frame.Error.Code != tc.wantCode {
				t.Fatalf("want error %s, got ok=%v error=%+v", tc.wantCode, frame.OK, frame.Error)
			}
		})
	}
}

func TestPairingUpdate_OK(t *testing.T) {
	frame := callPairingUpdate(t, nil)
	if !frame.OK {
		t.Fatalf("want ok, got error %+v", frame.Error)
	}
}
