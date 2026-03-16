package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

const (
	frameTypeStdin  byte = 0x00
	frameTypeData   byte = 0x01
	frameTypeResize byte = 0x02
	frameTypeExit   byte = 0x03
)

type exitFrame struct {
	Exited   bool `json:"exited"`
	ExitCode int  `json:"exit_code"`
}

type resizeFrame struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// attachTerminal connects to a sandbox terminal via WebSocket and provides an interactive session.
func attachTerminal(ctx context.Context, wsURL, apiKey string) (int, error) {
	dialer := websocket.Dialer{}
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+apiKey)

	conn, _, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return 1, fmt.Errorf("connecting to terminal: %w", err)
	}
	defer conn.Close()

	// Put terminal into raw mode
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return 1, fmt.Errorf("setting raw terminal mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	exitCodeCh := make(chan int, 1)

	// Mutex for concurrent writes to the websocket connection
	var wsMu sync.Mutex

	// Send initial resize
	sendResize(conn, &wsMu)

	// stdin → ws
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				cancel()
				return
			}
			frame := make([]byte, 1+n)
			frame[0] = frameTypeStdin
			copy(frame[1:], buf[:n])
			wsMu.Lock()
			err = conn.WriteMessage(websocket.BinaryMessage, frame)
			wsMu.Unlock()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// ws → stdout (streaming — reads frame type byte, then streams body to stdout)
	go func() {
		for {
			_, reader, err := conn.NextReader()
			if err != nil {
				cancel()
				return
			}

			// Read the first byte (frame type)
			var typeBuf [1]byte
			if _, err := io.ReadFull(reader, typeBuf[:]); err != nil {
				cancel()
				return
			}

			switch typeBuf[0] {
			case frameTypeData:
				// Stream directly to stdout without buffering the entire message
				if _, err := io.Copy(os.Stdout, reader); err != nil {
					cancel()
					return
				}
			case frameTypeExit:
				// Exit frames are small, safe to read fully
				payload, err := io.ReadAll(reader)
				if err != nil {
					exitCodeCh <- 0
					cancel()
					return
				}
				var ef exitFrame
				if err := json.Unmarshal(payload, &ef); err == nil {
					exitCodeCh <- ef.ExitCode
				} else {
					exitCodeCh <- 0
				}
				cancel()
				return
			default:
				// Drain unknown frame types
				io.Copy(io.Discard, reader)
			}
		}
	}()

	// SIGWINCH → ws (unix only; no-op on Windows)
	sigCh := make(chan os.Signal, 1)
	notifyResize(sigCh)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigCh:
				sendResize(conn, &wsMu)
			}
		}
	}()

	// Close the ws connection when context is cancelled (unblocks NextReader)
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	<-ctx.Done()

	select {
	case code := <-exitCodeCh:
		return code, nil
	default:
		return 0, nil
	}
}

func sendResize(conn *websocket.Conn, mu *sync.Mutex) {
	w, h, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return
	}
	rf := resizeFrame{Cols: w, Rows: h}
	payload, err := json.Marshal(rf)
	if err != nil {
		return
	}
	frame := make([]byte, 1+len(payload))
	frame[0] = frameTypeResize
	copy(frame[1:], payload)
	mu.Lock()
	_ = conn.WriteMessage(websocket.BinaryMessage, frame)
	mu.Unlock()
}
