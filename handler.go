package grpcmix

import (
	"net/http"
	"strings"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

type mixHandler struct {
	writer      http.ResponseWriter
	request     *http.Request
	grpcServer  *grpc.Server
	grpcWeb     *grpcweb.WrappedGrpcServer
	httpHandler http.Handler
}

func (h mixHandler) isGrpc() bool {
	return h.request.ProtoMajor >= 2 &&
		h.request.Method == http.MethodPost &&
		h.request.Header.Get("Content-Type") == "application/grpc"
}

func (h mixHandler) isRegisteredGrpcPath() bool {
	if h.grpcServer == nil {
		return false
	}
	path := h.request.URL.Path
	// path should like "/service/method"
	if len(path) < 4 {
		// minimal length is 4: "/a/b"
		return false
	}
	if path[0] != '/' {
		return false
	}

	// extract service and method
	serviceName, _, ok := strings.Cut(path[1:], "/")
	if !ok {
		return false
	}

	_, ok = h.grpcServer.GetServiceInfo()[serviceName]
	return ok
}

func (h mixHandler) handleGrpc() bool {
	if h.grpcServer == nil {
		return false
	}
	if h.isGrpc() && h.isRegisteredGrpcPath() {
		h.grpcServer.ServeHTTP(h.writer, h.request)
		return true
	}
	return false
}

func (h mixHandler) handleGrpcWeb() bool {
	if h.grpcWeb == nil {
		return false
	}
	if (h.grpcWeb.IsGrpcWebRequest(h.request) || h.grpcWeb.IsAcceptableGrpcCorsRequest(h.request)) && h.isRegisteredGrpcPath() {
		wrapper := wrapBrotli(h.writer, h.request)
		defer wrapper.Close()
		h.grpcWeb.ServeHTTP(wrapper, h.request)
		return true
	}
	return false
}

func (h mixHandler) handleHttp() bool {
	if h.httpHandler == nil {
		return false
	}
	h.httpHandler.ServeHTTP(h.writer, h.request)
	return true
}

func (h mixHandler) handle() {
	switch {
	case h.handleGrpc():
	case h.handleGrpcWeb():
	case h.handleHttp():
	default:
		http.NotFound(h.writer, h.request)
	}
}

func newHandler(grpcServer *grpc.Server, httpHandler http.Handler) http.Handler {
	grpcWeb := grpcweb.WrapServer(grpcServer, grpcweb.WithOriginFunc(func(origin string) bool {
		return true
	}))
	return h2c.NewHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		mixHandler{
			writer:      writer,
			request:     request,
			grpcServer:  grpcServer,
			grpcWeb:     grpcWeb,
			httpHandler: httpHandler,
		}.handle()
	}), &http2.Server{})
}
