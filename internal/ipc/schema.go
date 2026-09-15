package ipc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const ProtocolVersion = 1

type Request struct {
	Version    int    `json:"version"`
	Type       string `json:"type"`
	RequestID  string `json:"request_id"`
	Text       string `json:"text"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}

type Response struct {
	Version     int    `json:"version"`
	RequestID   string `json:"request_id"`
	OK          bool   `json:"ok"`
	Translation string `json:"translation"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	Error       string `json:"error,omitempty"`
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
