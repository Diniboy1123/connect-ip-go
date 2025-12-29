package connectip

import (
	"net/http"

	"github.com/apernet/quic-go/http3"
	"github.com/apernet/quic-go/quicvarint"
)

var contextIDZero = quicvarint.Append([]byte{}, 0)

type Proxy struct{}

func (s *Proxy) Proxy(w http.ResponseWriter, _ *Request) (*Conn, error) {
	w.Header().Set(http3.CapsuleProtocolHeader, capsuleProtocolHeaderValue)
	w.WriteHeader(http.StatusOK)

	str := w.(http3.HTTPStreamer).HTTPStream()
	return newProxiedConn(str), nil
}
