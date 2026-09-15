package ipc

import (
	"fmt"
	"net"
)

func SendRequest(socketPath string, req Request) (Response, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return Response{}, fmt.Errorf("connect to server: %w", err)
	}
	defer conn.Close()

	if err := WriteMessage(conn, req); err != nil {
		return Response{}, fmt.Errorf("send request: %w", err)
	}

	var resp Response
	if err := ReadMessage(conn, &resp); err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	return resp, nil
}
