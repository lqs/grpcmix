package grpcmix

import (
	"errors"
	"github.com/andybalholm/brotli"
	"io"
	"net/http"
)

type brotliWrapper struct {
	http.ResponseWriter
	request      *http.Request
	brotliWriter io.WriteCloser
	isClosed     bool
}

func (w *brotliWrapper) Write(data []byte) (int, error) {
	if w.isClosed {
		return 0, errors.New("brotliWrapper is closed")
	}
	if w.brotliWriter == nil {
		// create brotli writer on first write, because at this point the response headers are set but not yet sent
		w.brotliWriter = brotli.HTTPCompressor(w.ResponseWriter, w.request)
	}
	return w.brotliWriter.Write(data)
}

func (w *brotliWrapper) Close() {
	if w.isClosed {
		return
	}
	if w.brotliWriter != nil {
		_ = w.brotliWriter.Close()
	}
	w.isClosed = true
}

func wrapBrotli(writer http.ResponseWriter, request *http.Request) *brotliWrapper {
	return &brotliWrapper{
		ResponseWriter: writer,
		request:        request,
	}
}
