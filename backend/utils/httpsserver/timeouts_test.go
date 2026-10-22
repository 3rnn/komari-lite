package httpsserver

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHTTPTimeoutsBoundHeadersAndBodiesWithoutWriteDeadline(t *testing.T) {
	s := NewHTTPServer("127.0.0.1:0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" {
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			kind, data, err := conn.ReadMessage()
			if err == nil {
				_ = conn.WriteMessage(kind, data)
			}
			return
		}
		if r.URL.Path == "/stream" {
			time.Sleep(120 * time.Millisecond)
			_, _ = io.WriteString(w, "stream-alive")
			return
		}
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body timeout", http.StatusRequestTimeout)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	if s.ReadHeaderTimeout <= 0 || s.ReadTimeout <= 0 || s.IdleTimeout <= 0 {
		t.Fatal("missing HTTP read deadline")
	}
	if s.WriteTimeout != 0 {
		t.Fatal("server-wide write timeout breaks streams")
	}
	s.ReadHeaderTimeout = 40 * time.Millisecond
	s.ReadTimeout = 80 * time.Millisecond
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	go s.Serve(listener)
	dial := func() net.Conn {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		return conn
	}
	t.Run("headers", func(t *testing.T) {
		conn := dial()
		defer conn.Close()
		_, _ = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: local")
		buf := make([]byte, 256)
		_, err := conn.Read(buf)
		if err == nil {
			t.Fatal("incomplete headers did not close connection")
		}
	})
	t.Run("body", func(t *testing.T) {
		conn := dial()
		defer conn.Close()
		_, _ = io.WriteString(conn, "POST / HTTP/1.1\r\nHost: local\r\nContent-Length: 100\r\n\r\nx")
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusRequestTimeout {
			t.Fatalf("status = %d", resp.StatusCode)
		}
	})
	t.Run("websocket", func(t *testing.T) {
		conn, _, err := websocket.DefaultDialer.Dial("ws://"+listener.Addr().String()+"/ws", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		time.Sleep(120 * time.Millisecond)
		if err := conn.WriteMessage(websocket.TextMessage, []byte("still-alive")); err != nil {
			t.Fatal(err)
		}
		_, data, err := conn.ReadMessage()
		if err != nil || string(data) != "still-alive" {
			t.Fatalf("upgraded connection inherited HTTP read deadline: %q %v", data, err)
		}
	})
	t.Run("stream", func(t *testing.T) {
		conn := dial()
		defer conn.Close()
		_, _ = io.WriteString(conn, "GET /stream HTTP/1.1\r\nHost: local\r\nConnection: close\r\n\r\n")
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || !strings.Contains(string(body), "stream-alive") {
			t.Fatalf("stream = %q, %v", body, err)
		}
	})
}
