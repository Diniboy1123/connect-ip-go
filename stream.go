package connectip

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/quic-go/quicvarint"
)

// Stream is a generic interface that can work with both HTTP/2 and HTTP/3 streams
type Stream interface {
	io.ReadWriteCloser
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	Context() context.Context
	SendDatagram([]byte) error
	ReceiveDatagram(context.Context) ([]byte, error)
	CancelRead(code uint64)
}

// HTTP3Stream wraps an HTTP/3 stream to implement the generic Stream interface
type HTTP3Stream struct {
	stream http3.Stream
}

func NewHTTP3Stream(stream http3.Stream) *HTTP3Stream {
	return &HTTP3Stream{stream: stream}
}

func (s *HTTP3Stream) Read(p []byte) (n int, err error) {
	return s.stream.Read(p)
}

func (s *HTTP3Stream) Write(p []byte) (n int, err error) {
	return s.stream.Write(p)
}

func (s *HTTP3Stream) Close() error {
	return s.stream.Close()
}

func (s *HTTP3Stream) SetDeadline(t time.Time) error {
	return s.stream.SetDeadline(t)
}

func (s *HTTP3Stream) SetReadDeadline(t time.Time) error {
	return s.stream.SetReadDeadline(t)
}

func (s *HTTP3Stream) SetWriteDeadline(t time.Time) error {
	return s.stream.SetWriteDeadline(t)
}

func (s *HTTP3Stream) Context() context.Context {
	return s.stream.Context()
}

func (s *HTTP3Stream) SendDatagram(data []byte) error {
	return s.stream.SendDatagram(data)
}

func (s *HTTP3Stream) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	return s.stream.ReceiveDatagram(ctx)
}

func (s *HTTP3Stream) CancelRead(code uint64) {
	s.stream.CancelRead(quic.StreamErrorCode(code))
}

// HTTP2ResponseWriterStream implements io.ReadWriteCloser for HTTP/2 CONNECT responses
type HTTP2ResponseWriterStream struct {
	w   http.ResponseWriter
	req *http.Request
	ctx context.Context
}

func (s *HTTP2ResponseWriterStream) Read(p []byte) (n int, err error) {
	return s.req.Body.Read(p)
}

func (s *HTTP2ResponseWriterStream) Write(p []byte) (n int, err error) {
	return s.w.Write(p)
}

func (s *HTTP2ResponseWriterStream) Close() error {
	return s.req.Body.Close()
}

// HTTP2Stream wraps an HTTP/2 stream to implement the generic Stream interface
// It emulates datagram functionality by sending/receiving framed data over the stream
type HTTP2Stream struct {
	stream     io.ReadWriteCloser
	ctx        context.Context
	datagramCh chan []byte
	errorCh    chan error
	closeCh    chan struct{}
}

func NewHTTP2Stream(stream io.ReadWriteCloser, ctx context.Context) *HTTP2Stream {
	s := &HTTP2Stream{
		stream:     stream,
		ctx:        ctx,
		datagramCh: make(chan []byte, 10),
		errorCh:    make(chan error, 1),
		closeCh:    make(chan struct{}),
	}
	go s.readDatagrams()
	return s
}

func (s *HTTP2Stream) Read(p []byte) (n int, err error) {
	return s.stream.Read(p)
}

func (s *HTTP2Stream) Write(p []byte) (n int, err error) {
	return s.stream.Write(p)
}

func (s *HTTP2Stream) Close() error {
	select {
	case <-s.closeCh:
		// Already closed
	default:
		close(s.closeCh)
	}
	return s.stream.Close()
}

func (s *HTTP2Stream) SetDeadline(t time.Time) error {
	// HTTP/2 streams don't typically support deadlines directly
	// This is a limitation of the HTTP/2 implementation
	return nil
}

func (s *HTTP2Stream) SetReadDeadline(t time.Time) error {
	return nil
}

func (s *HTTP2Stream) SetWriteDeadline(t time.Time) error {
	return nil
}

func (s *HTTP2Stream) Context() context.Context {
	return s.ctx
}

func (s *HTTP2Stream) SendDatagram(data []byte) error {
	// Frame the data with length prefix and send over stream
	buf := make([]byte, 0, len(data)+quicvarint.Len(uint64(len(data))))
	buf = quicvarint.Append(buf, uint64(len(data)))
	buf = append(buf, data...)
	_, err := s.stream.Write(buf)
	return err
}

func (s *HTTP2Stream) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.closeCh:
		return nil, io.EOF
	case err := <-s.errorCh:
		return nil, err
	case data := <-s.datagramCh:
		return data, nil
	}
}

// readDatagrams runs in a goroutine to read framed datagrams from the stream
func (s *HTTP2Stream) readDatagrams() {
	defer close(s.datagramCh)
	r := quicvarint.NewReader(s.stream)
	
	for {
		select {
		case <-s.closeCh:
			return
		default:
		}
		
		length, err := quicvarint.Read(r)
		if err != nil {
			select {
			case s.errorCh <- err:
			case <-s.closeCh:
			}
			return
		}
		
		data := make([]byte, length)
		_, err = io.ReadFull(r, data)
		if err != nil {
			select {
			case s.errorCh <- err:
			case <-s.closeCh:
			}
			return
		}
		
		select {
		case s.datagramCh <- data:
		case <-s.closeCh:
			return
		}
	}
}

func (s *HTTP2Stream) CancelRead(code uint64) {
	// HTTP/2 doesn't have the same cancel mechanism as QUIC
	// We'll just close the channel to signal cancellation
	select {
	case <-s.closeCh:
		// Already closed
	default:
		close(s.closeCh)
	}
}