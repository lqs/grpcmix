package grpcmix

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	port := 30000 + rand.Intn(20000)
	server := NewServer(Config{
		Port:          port,
		ShutdownDelay: 2 * time.Second,
		OnStarted: func() {
			t.Logf("server started on port %d", port)
			response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
			if err != nil {
				t.Errorf("http.Get() error = %v", err)
				return
			}
			if response.StatusCode != http.StatusOK {
				t.Errorf("response.StatusCode = %d, want %d", response.StatusCode, http.StatusOK)
			}
			responseBoby, err := io.ReadAll(response.Body)
			if err != nil {
				t.Errorf("io.ReadAll() error = %v", err)
				return
			}
			if string(responseBoby) != "OK" {
				t.Errorf("responseBoby = %s, want %s", responseBoby, "OK")
			}
		},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.StartAndWait(ctx); err != nil {
		t.Errorf("server.StartAndWait() error = %v", err)
	}
}
