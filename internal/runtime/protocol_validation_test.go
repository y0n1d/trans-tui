package runtime

import (
	"strings"
	"testing"

	"github.com/y0n1d/trans-tui/internal/ipc"
)

// TestInvalidRequestRejectedThroughProductionPath pins P2-4 on the real
// wiring: a request with an unknown or empty type must be rejected by the
// production server (ipc.Server + runtime newIPCHandler, started by
// startProductionIPCServer) before the handler runs. The handler's final
// branch is an implicit translate, so before validation such a request would
// have called the provider and pushed a TranslationResultMsg into the TUI
// channel — OK=false plus a silent ipcCh proves that default branch was
// never taken.
func TestInvalidRequestRejectedThroughProductionPath(t *testing.T) {
	cases := []struct {
		name    string
		typ     string
		wantErr string
	}{
		{name: "unknown type", typ: "frobnicate", wantErr: "unknown request type"},
		{name: "empty type", typ: "", wantErr: "missing request type"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testProviderCfg("DEEPSEEK_API_KEY", "https://api.deepseek.com", "deepseek-flash")
			socketPath, ipcCh := startProductionIPCServer(t, cfg)

			resp, err := ipc.SendRequest(socketPath, ipc.Request{
				Version:    ipc.ProtocolVersion,
				Type:       tc.typ,
				RequestID:  "invalid-type",
				Text:       "Hello world",
				SourceLang: cfg.Translation.SourceLang,
				TargetLang: cfg.Translation.TargetLang,
			})
			if err != nil {
				t.Fatalf("SendRequest: %v", err)
			}
			if resp.OK {
				t.Fatalf("request with type %q must be rejected, got OK=true", tc.typ)
			}
			if !strings.Contains(resp.Error, tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", resp.Error, tc.wantErr)
			}
			if resp.RequestID != "invalid-type" {
				t.Errorf("response request_id = %q, want %q", resp.RequestID, "invalid-type")
			}

			// Any translate attempt — success or provider error — posts to
			// ipcCh before responding, so an empty channel means the
			// translate branch never ran.
			select {
			case msg := <-ipcCh:
				t.Errorf("production handler emitted %T for an invalid request type: %v", msg, msg)
			default:
			}
		})
	}
}
