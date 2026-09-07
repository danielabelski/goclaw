package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/agent"
	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/scheduler"
	"github.com/nextlevelbuilder/goclaw/internal/tools"
)

// A teammate run must ask the provider to stream. Without it a slow reasoning
// model holds a silent connection for the whole generation and
// ResponseHeaderTimeout kills the request before a single token is produced.
//
// The same test pins the other half of the contract: streaming here is for
// connection liveness only. The run is deliberately never registered with the
// channel manager, so HandleAgentEvent drops its chunks and nothing reaches a
// user incrementally — the task result keeps coming from the final RunResult.
func TestHandleTeammateMessageSchedulesStreamedRun(t *testing.T) {
	var gotReq agent.RunRequest
	ran := make(chan struct{})

	sched := scheduler.NewScheduler(
		scheduler.DefaultLanes(),
		scheduler.QueueConfig{
			Mode:          scheduler.QueueModeQueue,
			Cap:           1,
			Drop:          scheduler.DropOld,
			MaxConcurrent: 1,
		},
		func(_ context.Context, req agent.RunRequest) (*agent.RunResult, error) {
			gotReq = req
			close(ran)
			return &agent.RunResult{Content: "member deliverable"}, nil
		},
	)
	defer sched.Stop()

	channelMgr := channels.NewManager(nil)
	deps := &ConsumerDeps{
		Cfg:        &config.Config{},
		Sched:      sched,
		ChannelMgr: channelMgr,
	}

	msg := bus.InboundMessage{
		Channel:  tools.ChannelSystem,
		SenderID: "teammate:dashboard",
		AgentID:  "coder",
		Content:  "[Assigned task #1 (id: 00000000-0000-0000-0000-000000000001)]: build something",
		Metadata: map[string]string{
			tools.MetaOriginChannel: "telegram",
			tools.MetaOriginChatID:  "12345",
			tools.MetaFromAgent:     "brain",
			tools.MetaToAgent:       "coder",
		},
	}

	if !handleTeammateMessage(context.Background(), msg, deps) {
		t.Fatal("handleTeammateMessage() = false, want true for a teammate: message on the system channel")
	}

	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("teammate run was never scheduled")
	}

	if !gotReq.Stream {
		t.Error("teammate run requested a non-streamed provider call: a slow model then holds a silent " +
			"connection for the whole generation until ResponseHeaderTimeout kills it")
	}

	if delivered, last := channelMgr.InterimDeliverySnapshot(gotReq.RunID); delivered != 0 || last != "" {
		t.Errorf("teammate run is registered for channel delivery (delivered=%d, last=%q); "+
			"streamed chunks would reach a user incrementally", delivered, last)
	}
}
