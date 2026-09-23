package ipc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const ProtocolVersion = 1

// Request types.
const (
	TypeTranslate      = "translate"
	TypeStatus         = "status"
	TypeEnterInputMode = "enter_input_mode"
	TypeDisplayText    = "display_text"
)

// Capability strings returned in status response.
const (
	CapDisplayText = "display_text"
)

type Request struct {
	Version    int    `json:"version"`
	Type       string `json:"type"`
	RequestID  string `json:"request_id"`
	Text       string `json:"text"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}

type Response struct {
	Version      int      `json:"version"`
	RequestID    string   `json:"request_id"`
	OK           bool     `json:"ok"`
	Translation  string   `json:"translation"`
	Provider     string   `json:"provider"`
	Model        string   `json:"model"`
	Error        string   `json:"error,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// validateRequest enforces the wire contract on an inbound request: exactly
// the current ProtocolVersion and one of the defined request types. The
// server runs it before dispatching to a handler, so an unsupported version
// or an unknown/empty type — including the zero values of both fields when a
// peer omits them — is rejected with an explicit error response instead of
// reaching (or falling through) application code. It never changes the wire
// format: the rejection reuses the existing ok/error response fields.
func validateRequest(req Request) error {
	if req.Version != ProtocolVersion {
		return fmt.Errorf("unsupported request protocol version %d (supported: %d)", req.Version, ProtocolVersion)
	}
	switch req.Type {
	case TypeTranslate, TypeStatus, TypeEnterInputMode, TypeDisplayText:
		return nil
	case "":
		return fmt.Errorf("missing request type")
	default:
		return fmt.Errorf("unknown request type %q", req.Type)
	}
}

func WriteMessage(w io.Writer, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(data)))

	if _, err := w.Write(buf[:]); err != nil {
		return fmt.Errorf("write length prefix: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}

	return nil
}

func ReadMessage(r io.Reader, msg any) error {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return fmt.Errorf("read length prefix: %w", err)
	}

	length := binary.BigEndian.Uint32(buf[:])
	if length == 0 {
		return fmt.Errorf("empty message")
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return fmt.Errorf("read payload: %w", err)
	}

	if err := json.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("unmarshal message: %w", err)
	}

	return nil
}
