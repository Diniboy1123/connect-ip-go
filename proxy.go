package connectip

import (
	"net/http"

	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/quic-go/quicvarint"
)

var contextIDZero = quicvarint.Append([]byte{}, 0)

type Proxy struct{}

func (s *Proxy) Proxy(w http.ResponseWriter, r *http.Request, req *Request) (*Conn, error) {
	w.Header().Set(http3.CapsuleProtocolHeader, capsuleProtocolHeaderValue)
	w.WriteHeader(http.StatusOK)

	// Detect the protocol version from the request
	if h3Streamer, ok := w.(http3.HTTPStreamer); ok {
		// HTTP/3 path
		str := h3Streamer.HTTPStream()
		return newProxiedConn(NewHTTP3Stream(str)), nil
	} else {
		// HTTP/2 path - use the ResponseWriter as a stream
		// For HTTP/2 CONNECT, the ResponseWriter can be used for bidirectional communication
		httpStream := &HTTP2ResponseWriterStream{
			w:   w,
			req: r,
			ctx: r.Context(),
		}
		return newProxiedConn(NewHTTP2Stream(httpStream, r.Context())), nil
	}
}
