package runtime

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/core"
	"github.com/y0n1d/trans-tui/internal/ipc"
	"github.com/y0n1d/trans-tui/internal/translator"
	"github.com/y0n1d/trans-tui/ui/tui"
	"io"
	"net"
	"os"
	"os/signal"
	"regexp"
	"time"

	tea "charm.land/bubbletea/v2"
)

func newProvider(cfg config.Config) (translator.Translator, error) {
	switch cfg.Provider.Type {
	case "openai-compatible":
		if cfg.Provider.APIKeyEnv == "" {
			return nil, fmt.Errorf("openai-compatible requires provider.api_key_env")
		}
		return translator.NewOpenAICompatibleProvider(translator.OpenAICompatibleConfig{
			BaseURL:    cfg.Provider.OpenAI.BaseURL,
			Model:      cfg.Provider.OpenAI.Model,
			APIKeyEnv:  cfg.Provider.APIKeyEnv,
			TimeoutSec: cfg.Provider.Timeout,
		}), nil
	case "google":
		apiKeyEnv := cfg.Provider.APIKeyEnv
		if apiKeyEnv == "" {
			apiKeyEnv = "GOOGLE_TRANSLATE_API_KEY"
		}
		return translator.NewGoogleProvider("", apiKeyEnv, cfg.Provider.Timeout), nil
	case "deepl":
		apiKeyEnv := cfg.Provider.APIKeyEnv
		if apiKeyEnv == "" {
			apiKeyEnv = "DEEPL_API_KEY"
		}
		return translator.NewDeepLProvider(
			cfg.Provider.DeepL.BaseURL,
			apiKeyEnv,
			cfg.Provider.Timeout,
		), nil
	case "libretranslate":
		return translator.NewLibreTranslateProvider(
			cfg.Provider.LibreTranslate.BaseURL,
			cfg.Provider.LibreTranslate.APIKeyEnv,
			cfg.Provider.Timeout,
		), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Provider.Type)
	}
}

func SocketPath() string {
	return config.DefaultConfig().SocketPath
}

func isAlive(socketPath string) bool {
	conn, err := (&net.Dialer{Timeout: 500 * time.Millisecond}).Dial("unix", socketPath)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func cleanup(socketPath string) {
	os.Remove(socketPath)
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = io.ReadFull(rand.Reader, b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// IsRunning reports whether a trans-tui server is currently reachable on the
// socket implied by cfg. Launchers use this to decide whether to create a new
// foot window or send the request directly via the existing server.
func IsRunning(cfg config.Config) bool {
	return isAlive(cfg.SocketPath)
}

func Run(text string, inputInitial, displayMode bool, cfg config.Config) {
	socketPath := cfg.SocketPath

	if isAlive(socketPath) {
		runClient(socketPath, text, inputInitial, displayMode, cfg)
		return
	}

	cleanup(socketPath)

	runServer(text, inputInitial, displayMode, cfg)
}

// runClient is the CLI lifecycle around clientExchange: it wires the real
// process streams in and turns a failed exchange into the historical
// "Error: ..." stderr message with exit status 1. Everything else lives in
// clientExchange so tests can exercise the real client path without
// os.Exit.
func runClient(socketPath, text string, inputInitial, displayMode bool, cfg config.Config) {
	if err := clientExchange(os.Stdout, socketPath, text, inputInitial, displayMode, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// clientExchange performs the whole client-side conversation with an already
// running server: the status probe, the config-fingerprint validation, and
// the follow-up request for the requested mode. Success output for translate
// mode is written to stdout; every failure is returned as an error instead of
// exiting, so tests cover this production code directly.
func clientExchange(stdout io.Writer, socketPath, text string, inputInitial, displayMode bool, cfg config.Config) error {
	statusID := generateID()
	statusResp, err := ipc.SendRequest(socketPath, ipc.Request{
		Version:   ipc.ProtocolVersion,
		Type:      ipc.TypeStatus,
		RequestID: statusID,
	})
	if err != nil {
		return err
	}
	if !statusResp.OK {
		return errors.New(statusResp.Error)
	}
	serverFingerprint := statusResp.Translation
	validFingerprint := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !validFingerprint.MatchString(serverFingerprint) {
		return errors.New("existing server uses an incompatible version.\nStop the running server and restart.")
	}
	clientFingerprint := cfg.Fingerprint()
	if serverFingerprint != clientFingerprint {
		return errors.New("existing server uses a different configuration.\nStop the running server or use matching configuration.")
	}

	if inputInitial {
		reqID := generateID()
		_, err := ipc.SendRequest(socketPath, ipc.Request{
			Version:   ipc.ProtocolVersion,
			Type:      ipc.TypeEnterInputMode,
			RequestID: reqID,
		})
		if err != nil {
			return err
		}
		return nil
	}

	if displayMode {
		// Check if server supports display_text capability.
		hasCap := false
		for _, cap := range statusResp.Capabilities {
			if cap == ipc.CapDisplayText {
				hasCap = true
				break
			}
		}
		if !hasCap {
			return errors.New("existing server does not support --display mode.\nStop the running server and restart with the latest version.")
		}

		reqID := generateID()
		resp, err := ipc.SendRequest(socketPath, ipc.Request{
			Version:   ipc.ProtocolVersion,
			Type:      ipc.TypeDisplayText,
			RequestID: reqID,
			Text:      text,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return errors.New(resp.Error)
		}
		return nil
	}

	reqID := generateID()
	resp, err := ipc.SendRequest(socketPath, ipc.Request{
		Version:    ipc.ProtocolVersion,
		Type:       ipc.TypeTranslate,
		RequestID:  reqID,
		Text:       text,
		SourceLang: cfg.Translation.SourceLang,
		TargetLang: cfg.Translation.TargetLang,
	})
	if err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	fmt.Fprintf(stdout, "%s\n", resp.Translation)
	return nil
}

func newIPCHandler(cfg config.Config, svc *core.Service, ipcCh chan<- tea.Msg) func(context.Context, ipc.Request) ipc.Response {
	return func(ctx context.Context, req ipc.Request) ipc.Response {
		if req.Type == ipc.TypeStatus {
			return ipc.Response{
				Version:      ipc.ProtocolVersion,
				RequestID:    req.RequestID,
				OK:           true,
				Translation:  cfg.Fingerprint(),
				Capabilities: []string{ipc.CapDisplayText},
			}
		}

		if req.Type == ipc.TypeEnterInputMode {
			ipcCh <- core.EnterInputModeMsg{}
			return ipc.Response{
				Version:   ipc.ProtocolVersion,
				RequestID: req.RequestID,
				OK:        true,
			}
		}

		if req.Type == ipc.TypeDisplayText {
			ipcCh <- core.DisplayTextMsg{
				RequestID: req.RequestID,
				Text:      req.Text,
			}
			return ipc.Response{
				Version:   ipc.ProtocolVersion,
				RequestID: req.RequestID,
				OK:        true,
			}
		}

		result, err := svc.Translate(ctx, translator.TranslationRequest{
			Text:       req.Text,
			SourceLang: req.SourceLang,
			TargetLang: req.TargetLang,
		})
		if err != nil {
			ipcCh <- core.TranslationErrorMsg{
				RequestID:  req.RequestID,
				Source:     req.Text,
				Error:      err.Error(),
				SourceLang: req.SourceLang,
				TargetLang: req.TargetLang,
			}
			return ipc.Response{
				Version:   ipc.ProtocolVersion,
				RequestID: req.RequestID,
				OK:        false,
				Error:     err.Error(),
			}
		}
		ipcCh <- core.TranslationResultMsg{
			RequestID:   req.RequestID,
			Source:      req.Text,
			Translation: result.Translation,
			SourceLang:  req.SourceLang,
			TargetLang:  result.TargetLang,
			Provider:    result.Provider,
			Model:       result.Model,
		}
		return ipc.Response{
			Version:     ipc.ProtocolVersion,
			RequestID:   req.RequestID,
			OK:          true,
			Translation: result.Translation,
			Provider:    result.Provider,
			Model:       result.Model,
		}
	}
}

// startIPCServer binds the IPC socket and starts serving in the background.
// The bind is synchronous, so a listen failure is returned to the caller
// before any TUI starts: the process must never keep running as if a server
// were reachable when no socket exists. Serve only returns once ctx is
// cancelled — the normal shutdown path — so a spontaneous return is reported
// instead of silently dropped.
func startIPCServer(ctx context.Context, socketPath string, handler ipc.Handler) error {
	server := ipc.NewServer(socketPath, handler)
	if err := server.Listen(ctx); err != nil {
		return err
	}
	go func() {
		if err := server.Serve(ctx); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "Error: IPC server stopped: %v\n", err)
		}
	}()
	return nil
}

func runServer(text string, inputInitial, displayMode bool, cfg config.Config) {
	prov, err := newProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	svc := core.NewService(prov)

	ipcCh := make(chan tea.Msg, 10)

	handler := newIPCHandler(cfg, svc, ipcCh)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := startIPCServer(ctx, cfg.SocketPath, handler); err != nil {
		// Fail before the TUI starts: without a listening socket the TUI
		// would run while launchers and second invocations see no server.
		fmt.Fprintf(os.Stderr, "Error: IPC server: %v\n", err)
		os.Exit(1)
	}

	initialState := core.AppState{}

	km := tui.NewKeyMapFromBindings(
		cfg.KeyBindings.Quit,
		cfg.KeyBindings.ManualInput,
	)

	tuiModel := tui.New(initialState, svc, text, cfg.Translation.SourceLang, cfg.Translation.TargetLang, inputInitial, displayMode, km)

	p := tea.NewProgram(tuiModel)

	go func() {
		for msg := range ipcCh {
			p.Send(msg)
		}
	}()

	go func() {
		<-ctx.Done()
		cleanup(cfg.SocketPath)
	}()

	// The initial translation is not sent from here anymore: the TUI schedules
	// it itself from its first WindowSizeMsg, i.e. once the viewport exists
	// (ui/tui scheduleInitialTranslation). A goroutine with a fixed delay only
	// guessed that condition and raced Program.Send against the program's own
	// initial resize message.

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cleanup(cfg.SocketPath)
}
