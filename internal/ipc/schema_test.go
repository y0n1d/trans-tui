package ipc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestWriteReadRequestRoundTrip(t *testing.T) {
	original := Request{
		Version:    ProtocolVersion,
		Type:       "translate",
		RequestID:  "550e8400-e29b-41d4-a716-446655440000",
		Text:       "Hello world",
		SourceLang: "auto",
		TargetLang: "zh",
	}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, original); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	var decoded Request
	if err := ReadMessage(&buf, &decoded); err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n  got  %+v\n  want %+v", decoded, original)
	}
}

func TestWriteReadResponseRoundTrip(t *testing.T) {
	original := Response{
		Version:     ProtocolVersion,
		RequestID:   "550e8400-e29b-41d4-a716-446655440000",
		OK:          true,
		Translation: "你好，世界",
		Provider:    "openai-compatible",
		Model:       "gpt-4o-mini",
	}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, original); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	var decoded Response
	if err := ReadMessage(&buf, &decoded); err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n  got  %+v\n  want %+v", decoded, original)
	}
}

func TestResponseWithError(t *testing.T) {
	original := Response{
		Version:   ProtocolVersion,
		RequestID: "abc-123",
		OK:        false,
		Error:     "missing API key",
	}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, original); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	var decoded Response
	if err := ReadMessage(&buf, &decoded); err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	if decoded.OK != false {
		t.Error("expected OK=false")
	}
	if decoded.Error != "missing API key" {
		t.Errorf("error = %q, want %q", decoded.Error, "missing API key")
	}
}

func TestLengthPrefixEncoding(t *testing.T) {
	payload := []byte(`{"version":1}`)
	var buf bytes.Buffer

	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	buf.Write(lenBuf[:])
	buf.Write(payload)

	raw := buf.Bytes()
	gotLen := binary.BigEndian.Uint32(raw[:4])
	if gotLen != uint32(len(payload)) {
		t.Errorf("length prefix = %d, want %d", gotLen, len(payload))
	}
}

func TestReadMessageEmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], 0)
	buf.Write(lenBuf[:])

	var msg Request
	err := ReadMessage(&buf, &msg)
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestWriteReadMultipleMessages(t *testing.T) {
	req1 := Request{Version: 1, Type: "translate", RequestID: "1", Text: "hello", SourceLang: "en", TargetLang: "fr"}
	req2 := Request{Version: 1, Type: "translate", RequestID: "2", Text: "world", SourceLang: "en", TargetLang: "de"}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, req1); err != nil {
		t.Fatalf("WriteMessage 1: %v", err)
	}
	if err := WriteMessage(&buf, req2); err != nil {
		t.Fatalf("WriteMessage 2: %v", err)
	}

	var decoded1, decoded2 Request
	if err := ReadMessage(&buf, &decoded1); err != nil {
		t.Fatalf("ReadMessage 1: %v", err)
	}
	if err := ReadMessage(&buf, &decoded2); err != nil {
		t.Fatalf("ReadMessage 2: %v", err)
	}

	if decoded1.RequestID != "1" {
		t.Errorf("decoded1.RequestID = %q, want %q", decoded1.RequestID, "1")
	}
	if decoded2.RequestID != "2" {
		t.Errorf("decoded2.RequestID = %q, want %q", decoded2.RequestID, "2")
	}
}

func TestInvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte("not json")
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	buf.Write(lenBuf[:])
	buf.Write(payload)

	var msg Request
	err := ReadMessage(&buf, &msg)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestResponseFieldJSONTags(t *testing.T) {
	resp := Response{
		Version:     1,
		RequestID:   "req-1",
		OK:          true,
		Translation: "translated",
		Provider:    "openai-compatible",
		Model:       "gpt-4o-mini",
		Error:       "",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	expected := map[string]any{
		"version":     float64(1),
		"request_id":  "req-1",
		"ok":          true,
		"translation": "translated",
		"provider":    "openai-compatible",
		"model":       "gpt-4o-mini",
	}

	for key, want := range expected {
		if m[key] != want {
			t.Errorf("field %q = %v, want %v", key, m[key], want)
		}
	}

	if _, exists := m["error"]; exists {
		t.Error("error field should be omitted when empty")
	}
}

func TestRequestFieldJSONTags(t *testing.T) {
	req := Request{
		Version:    1,
		Type:       "translate",
		RequestID:  "req-1",
		Text:       "hello",
		SourceLang: "en",
		TargetLang: "fr",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	expected := map[string]any{
		"version":     float64(1),
		"type":        "translate",
		"request_id":  "req-1",
		"text":        "hello",
		"source_lang": "en",
		"target_lang": "fr",
	}

	for key, want := range expected {
		if m[key] != want {
			t.Errorf("field %q = %v, want %v", key, m[key], want)
		}
	}
}
