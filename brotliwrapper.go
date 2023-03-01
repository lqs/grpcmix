package grpcmix

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
	"golang.org/x/net/http/httpguts"
)

type brotliWrapper struct {
	http.ResponseWriter
	request *http.Request
	writer  io.Writer
}

const minimumSizeToCompress = 512

type CompressionType int

const (
	compressionTypeNone CompressionType = iota
	compressionTypeBrotli
	compressionTypeGzip
)

func (w *brotliWrapper) checkCompressionType(data []byte) CompressionType {
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/grpc-web") {
		return compressionTypeNone
	}
	if len(data) < 5 {
		// not a grpc-web header
		return compressionTypeNone
	}
	size := uint32(data[1])<<24 | uint32(data[2])<<16 | uint32(data[3])<<8 | uint32(data[4])
	if size < minimumSizeToCompress {
		return compressionTypeNone
	}
	acceptEncoding := w.request.Header.Values("Accept-Encoding")
	switch {
	case httpguts.HeaderValuesContainsToken(acceptEncoding, "br"):
		return compressionTypeBrotli
	case httpguts.HeaderValuesContainsToken(acceptEncoding, "gzip"):
		return compressionTypeGzip
	default:
		return compressionTypeNone
	}
}

func (w *brotliWrapper) Write(data []byte) (int, error) {
	if w.writer == nil {
		compressionType := w.checkCompressionType(data)
		switch compressionType {
		case compressionTypeBrotli:
			w.writer = brotli.NewWriterOptions(w.ResponseWriter, brotli.WriterOptions{
				Quality: brotli.DefaultCompression,
				LGWin:   16,
			})
			w.Header().Set("Content-Encoding", "br")
		case compressionTypeGzip:
			var err error
			w.writer, err = gzip.NewWriterLevel(w.ResponseWriter, gzip.DefaultCompression)
			if err != nil {
				return 0, err
			}
			w.Header().Set("Content-Encoding", "gzip")
		default:
			w.writer = w.ResponseWriter
		}
		if !httpguts.HeaderValuesContainsToken(w.Header().Values("Vary"), "Accept-Encoding") {
			w.Header().Add("Vary", "Accept-Encoding")
		}
	}
	return w.writer.Write(data)
}

func (w *brotliWrapper) Close() {
	if closer, ok := w.writer.(io.Closer); ok {
		_ = closer.Close()
	}
}

func wrapBrotli(writer http.ResponseWriter, request *http.Request) *brotliWrapper {
	return &brotliWrapper{
		ResponseWriter: writer,
		request:        request,
	}
}
