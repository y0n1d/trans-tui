package ipc

import (
	"fmt"
	"net"
	"time"
)

// SendRequest performs one request/response exchange over the unix socket.
// The connection carries a deadline (defaultConnDeadline) covering both the
// write and the read, so a hung server cannot block the caller forever. The
// deadline is well above the provider timeout (default 30s) that bounds a
// server-side translate, so slow-but-valid responses are never cut off.
func SendRequest(socketPath string, req Request) (Response, error) {
	return sendRequest(socketPath, req, defaultConnDeadline)
}

// sendRequest is SendRequest with an explicit deadline (unexported so tests
// can exercise the deadline without waiting minutes).
func sendRequest(socketPath string, req Request, deadline time.Duration) (Response, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return Response{}, fmt.Errorf("connect to server: %w", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(deadline))

	if err := WriteMessage(conn, req); err != nil {
		return Response{}, fmt.Errorf("send request: %w", err)
	}

	var resp Response
	if err := ReadMessage(conn, &resp); err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	return resp, nil
}
