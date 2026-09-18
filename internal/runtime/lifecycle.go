package runtime

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"my-trans/internal/config"
	"my-trans/internal/core"
	"my-trans/internal/ipc"
	"my-trans/internal/translator"
	"my-trans/ui/tui"
	"net"
	"os"
	"os/signal"
	"regexp"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

func Run(text string, cfg config.Config) {
	socketPath := cfg.SocketPath

	if isAlive(socketPath) {
		runClient(socketPath, text, cfg)
		return
	}

	cleanup(socketPath)

	runServer(text, cfg)
}

func runClient(socketPath, text string, cfg config.Config) {
	statusID := generateID()
	statusResp, err := ipc.SendRequest(socketPath, ipc.Request{
		Version:   ipc.ProtocolVersion,
		Type:      "status",
		RequestID: statusID,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !statusResp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", statusResp.Error)
		os.Exit(1)
	}
	serverFingerprint := statusResp.Translation
	validFingerprint := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !validFingerprint.MatchString(serverFingerprint) {
		fmt.Fprintf(os.Stderr, "Error: existing server uses an incompatible version.\nStop the running server and restart.\n")
		os.Exit(1)
	}
	clientFingerprint := cfg.Fingerprint()
	if serverFingerprint != clientFingerprint {
		fmt.Fprintf(os.Stderr, "Error: existing server uses a different configuration.\nStop the running server or use matching configuration.\n")
		os.Exit(1)
	}

	reqID := generateID()
	resp, err := ipc.SendRequest(socketPath, ipc.Request{
		Version:    ipc.ProtocolVersion,
		Type:       "translate",
		RequestID:  reqID,
		Text:       text,
		SourceLang: cfg.Translation.SourceLang,
		TargetLang: cfg.Translation.TargetLang,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}
	fmt.Println(resp.Translation)
}

func runServer(text string, cfg config.Config) {
	prov, err := newProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	svc := core.NewService(prov)

	ipcCh := make(chan tea.Msg, 10)

	handler := func(ctx context.Context, req ipc.Request) ipc.Response {
		if req.Type == "status" {
			return ipc.Response{
				Version:     ipc.ProtocolVersion,
				RequestID:   req.RequestID,
				OK:          true,
				Translation: cfg.Fingerprint(),
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
			TargetLang:  req.TargetLang,
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	server := ipc.NewServer(cfg.SocketPath, handler)
	go server.ListenAndServe(ctx)

	initialState := core.AppState{}
	tuiModel := tui.New(initialState, svc, text, cfg.Translation.SourceLang, cfg.Translation.TargetLang)

	p := tea.NewProgram(tuiModel, tea.WithAltScreen(), tea.WithMouseCellMotion())

	go func() {
		for msg := range ipcCh {
			p.Send(msg)
		}
	}()

	go func() {
		<-ctx.Done()
		cleanup(cfg.SocketPath)
	}()

	go func() {
		time.Sleep(200 * time.Millisecond)
		p.Send(tui.InitialTranslationMsg{})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cleanup(cfg.SocketPath)
}
