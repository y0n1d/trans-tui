//go:build ignore

# Contract: IPC Message Schema

**Module**: `internal/ipc`
**Type**: Unix Domain Socket protocol

## Transport

- **Socket path**: `$XDG_RUNTIME_DIR/my-trans.sock`
- **Protocol**: Length-prefixed JSON
- **Framing**: 4-byte big-endian uint32 length header, followed by JSON payload

## Message Format

### Request (CLI → Server)

```json
{
  "version": 1,
  "type": "translate",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "text": "Hello world",
  "source_lang": "auto",
  "target_lang": "zh"
}
```

| Field         | Type   | Required | Description                        |
|---------------|--------|----------|------------------------------------|
| version       | int    | yes      | Protocol version (currently 1)     |
| type          | string | yes      | Message type: "translate"          |
| request_id    | string | yes      | Unique ID for request/response matching |
| text          | string | yes      | Text to translate                  |
| source_lang   | string | yes      | Source language code or "auto"     |
| target_lang   | string | yes      | Target language code               |

### Response (Server → CLI)

```json
{
  "version": 1,
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "ok": true,
  "translation": "你好，世界",
  "provider": "openai-compatible",
  "model": "gpt-4o-mini",
  "error": ""
}
```

| Field         | Type   | Required | Description                        |
|---------------|--------|----------|------------------------------------|
| version       | int    | yes      | Protocol version (currently 1)     |
| request_id    | string | yes      | Matches the request ID             |
| ok            | bool   | yes      | True if translation succeeded      |
| translation   | string | yes      | Translated text (empty on error)   |
| provider      | string | yes      | Provider name                      |
| model         | string | yes      | Model name                         |
| error         | string | no       | Error message (empty on success)   |

## Protocol Rules

1. **Version field**: MUST be present in all messages. Current version: 1.
2. **Request ID**: Server MUST echo the request ID in the response for matching.
3. **Single request**: Client sends one request, waits for one response, then closes its connection.
4. **Error handling**: If the server cannot process the request, it returns `ok: false` with an `error` message.
5. **Socket cleanup**: Server MUST remove the socket file on shutdown.

## Future Message Types

The `type` field allows adding new message types (e.g., "ocr", "tts", "retry") without breaking the protocol. Version 1 supports only "translate".
