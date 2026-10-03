package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/core"
	"github.com/y0n1d/trans-tui/internal/ipc"
	"github.com/y0n1d/trans-tui/internal/translator"

	tea "charm.land/bubbletea/v2"
)

// TestIPCHandlerTranslateEmitsStartedBeforeError pins the failure half of the
// started-before-outcome ordering: a failing TypeTranslate still pushes
// TranslationStartedMsg before it translates and TranslationErrorMsg after.
func TestIPCHandlerTranslateEmitsStartedBeforeError(t *testing.T) {
	stub := &stubTranslator{err: errors.New("provider boom")}
	svc := core.NewService(stub)
	ch := make(chan tea.Msg, 4)
	handler := newIPCHandler(config.DefaultConfig(), svc, ch)

	resp := handler(context.Background(), ipc.Request{
		Version:    ipc.ProtocolVersion,
		Type:       ipc.TypeTranslate,
		RequestID:  "req-err",
		Text:       "Hello",
		SourceLang: "auto",
		TargetLang: "auto",
	})
	if resp.OK {
		t.Fatal("response should not be OK for a failing provider")
	}

	msg := <-ch
	started, ok := msg.(core.TranslationStartedMsg)
	if !ok {
		t.Fatalf("first handler message = %T, want core.TranslationStartedMsg", msg)
	}
	if started.RequestID != "req-err" {
		t.Errorf("started request id = %q, want %q", started.RequestID, "req-err")
	}

	msg = <-ch
	errMsg, ok := msg.(core.TranslationErrorMsg)
	if !ok {
		t.Fatalf("second handler message = %T, want core.TranslationErrorMsg", msg)
	}
	if errMsg.Error != "provider boom" {
		t.Errorf("error message = %q, want %q", errMsg.Error, "provider boom")
	}
}

// TestIPCHandlerDisplayTextDoesNotEmitStarted pins the display-only boundary:
// TypeDisplayText must only push DisplayTextMsg — never TranslationStartedMsg,
// because displaying OCR text is not a translation and must not raise the
// "Translating..." indicator (grim-ocr-display semantics).
func TestIPCHandlerDisplayTextDoesNotEmitStarted(t *testing.T) {
	stub := &stubTranslator{
		result: translator.TranslationResult{Translation: "t", Provider: "p", Model: "m"},
	}
	svc := core.NewService(stub)
	ch := make(chan tea.Msg, 2)
	handler := newIPCHandler(config.DefaultConfig(), svc, ch)

	resp := handler(context.Background(), ipc.Request{
		Version:   ipc.ProtocolVersion,
		Type:      ipc.TypeDisplayText,
		RequestID: "disp-1",
		Text:      "OCR text",
	})
	if !resp.OK {
		t.Fatalf("response not OK: %s", resp.Error)
	}

	select {
	case msg := <-ch:
		if _, ok := msg.(core.DisplayTextMsg); !ok {
			t.Fatalf("first handler message = %T, want core.DisplayTextMsg", msg)
		}
	default:
		t.Fatal("handler did not emit a DisplayTextMsg")
	}
	select {
	case msg := <-ch:
		t.Fatalf("handler emitted %T for display_text; only DisplayTextMsg is allowed", msg)
	default:
	}
}
