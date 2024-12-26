package grpcmix

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server interface {
	grpc.ServiceRegistrar
	reflection.GRPCServer
	GetConnStateMap() map[net.Conn]http.ConnState
	StartAndWait(ctx context.Context) error
}

type Config struct {
	Port                int
	ShutdownDelay       time.Duration
	MaxHeaderBytes      int
	GrpcServerOptions   []grpc.ServerOption
	OnStarted           func()
	AllowPrivateNetwork bool
	HTTP2Server         *http2.Server
}

type server struct {
	config       Config
	connStateMap map[net.Conn]http.ConnState
	mutex        sync.RWMutex // protects connStateMap
	grpcServer   *grpc.Server
	httpHandler  http.Handler
}

func (s *server) GetServiceInfo() map[string]grpc.ServiceInfo {
	if s.grpcServer == nil {
		return nil
	}
	return s.grpcServer.GetServiceInfo()
}

func (s *server) GetConnStateMap() map[net.Conn]http.ConnState {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	connStateMap := make(map[net.Conn]http.ConnState, len(s.connStateMap))
	for conn, state := range s.connStateMap {
		connStateMap[conn] = state
	}
	return connStateMap
}

func (s *server) StartAndWait(ctx context.Context) error {
	var connectionClose atomic.Bool

	http2Server := s.config.HTTP2Server
	if http2Server == nil {
		http2Server = &http2.Server{}
	}
	handler, wait := s.newHandler(http2Server, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if connectionClose.Load() {
			w.Header().Set("Connection", "close")
		}
		s.httpHandler.ServeHTTP(w, r)
	}))
	httpServer := &http.Server{
		Handler:        handler,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
		ConnState:      s.updateConnState,
	}
	if err := http2.ConfigureServer(httpServer, http2Server); err != nil {
		return fmt.Errorf("failed to configure HTTP/2 server: %v", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: s.config.Port})
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	done := make(chan error)
	go func() {
		defer close(done)
		done <- httpServer.Serve(listener)
	}()

	if s.config.OnStarted != nil {
		s.config.OnStarted()
	}

	select {
	case <-ctx.Done():
		// don't call server.SetKeepAlivesEnabled here because it will close idle connections immediately
		connectionClose.Store(true)
		// round up to integer second
		shutdownDelay := (s.config.ShutdownDelay + time.Second - time.Nanosecond).Truncate(time.Second)
		for shutdownDelay > 0 {
			// check for connections every 100ms for total 1s. if all checks are negative, shutdown the server
			hasConnections := len(s.GetConnStateMap()) > 0
			for i := 0; i < 10; i++ {
				time.Sleep(100 * time.Millisecond)
				hasConnections = hasConnections || len(s.GetConnStateMap()) > 0
			}
			shutdownDelay -= time.Second
			if !hasConnections {
				break
			}
		}
		_ = httpServer.Shutdown(context.Background())
		wait()
		<-done
		return nil
	case err := <-done:
		// server.Serve() returned an error before context was canceled
		return fmt.Errorf("failed to start server: %v", err)
	}
}

func (s *server) updateConnState(conn net.Conn, state http.ConnState) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	switch state {
	case http.StateNew:
		s.connStateMap[conn] = state
	case http.StateActive, http.StateIdle:
		if _, ok := s.connStateMap[conn]; ok {
			s.connStateMap[conn] = state
		}
	case http.StateHijacked, http.StateClosed:
		delete(s.connStateMap, conn)
	}
}

func (s *server) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.grpcServer.RegisterService(desc, impl)
}

// NewServer creates a new Server instance.
func NewServer(config Config, httpHandler http.Handler) Server {
	grpcServer := grpc.NewServer(config.GrpcServerOptions...)
	return &server{
		config:       config,
		connStateMap: make(map[net.Conn]http.ConnState),
		grpcServer:   grpcServer,
		httpHandler:  httpHandler,
	}
}
