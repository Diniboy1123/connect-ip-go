package connectip

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/require"
	"github.com/yosida95/uritemplate/v3"
)

// mockHTTP2Stream implements io.ReadWriteCloser for testing HTTP/2 streams
type mockHTTP2Stream struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	closed   bool
}

func newMockHTTP2Stream() *mockHTTP2Stream {
	return &mockHTTP2Stream{
		readBuf:  &bytes.Buffer{},
		writeBuf: &bytes.Buffer{},
	}
}

func (m *mockHTTP2Stream) Read(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.EOF
	}
	return m.readBuf.Read(p)
}

func (m *mockHTTP2Stream) Write(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.writeBuf.Write(p)
}

func (m *mockHTTP2Stream) Close() error {
	m.closed = true
	return nil
}

func TestHTTP2Stream(t *testing.T) {
	t.Run("basic read/write", func(t *testing.T) {
		mockStream := newMockHTTP2Stream()
		ctx := context.Background()
		stream := NewHTTP2Stream(mockStream, ctx)
		defer stream.Close()

		// Test writing
		testData := []byte("test data")
		n, err := stream.Write(testData)
		require.NoError(t, err)
		require.Equal(t, len(testData), n)

		// Verify data was written to the mock stream
		writtenData := mockStream.writeBuf.Bytes()
		require.Equal(t, testData, writtenData)

		// For reading, we need to put data in the read buffer first
		mockStream.readBuf.Write(testData)
		readBuf := make([]byte, len(testData))
		n, err = stream.Read(readBuf)
		require.NoError(t, err)
		require.Equal(t, len(testData), n)
		require.Equal(t, testData, readBuf)
	})

	t.Run("datagram functionality", func(t *testing.T) {
		mockStream := newMockHTTP2Stream()
		ctx := context.Background()
		stream := NewHTTP2Stream(mockStream, ctx)
		defer stream.Close()

		// Write some framed data to simulate incoming datagrams
		testData := []byte("test datagram")
		err := stream.SendDatagram(testData)
		require.NoError(t, err)

		// The written data should be in the write buffer with length prefix
		writtenData := mockStream.writeBuf.Bytes()
		require.True(t, len(writtenData) > len(testData), "written data should include length prefix")

		// Simulate the data being received by copying to read buffer
		mockStream.readBuf = bytes.NewBuffer(writtenData)

		// Now read the datagram
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		receivedData, err := stream.ReceiveDatagram(ctx)
		require.NoError(t, err)
		require.Equal(t, testData, receivedData)
	})

	t.Run("context cancellation", func(t *testing.T) {
		mockStream := newMockHTTP2Stream()
		ctx := context.Background()
		stream := NewHTTP2Stream(mockStream, ctx)
		defer stream.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := stream.ReceiveDatagram(ctx)
		require.Error(t, err)
		require.Equal(t, context.Canceled, err)
	})

	t.Run("close behavior", func(t *testing.T) {
		mockStream := newMockHTTP2Stream()
		ctx := context.Background()
		stream := NewHTTP2Stream(mockStream, ctx)

		// Close the stream
		err := stream.Close()
		require.NoError(t, err)

		// ReceiveDatagram should return EOF after close
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		_, err = stream.ReceiveDatagram(ctx)
		require.Error(t, err)
		require.Equal(t, io.EOF, err)
	})

	t.Run("cancel read", func(t *testing.T) {
		mockStream := newMockHTTP2Stream()
		ctx := context.Background()
		stream := NewHTTP2Stream(mockStream, ctx)

		// Cancel read
		stream.CancelRead(123)

		// ReceiveDatagram should return EOF after cancel
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		_, err := stream.ReceiveDatagram(ctx)
		require.Error(t, err)
		require.Equal(t, io.EOF, err)
	})
}

// mockHTTP3StreamForTest implements http3.Stream for testing
type mockHTTP3StreamForTest struct {
	*mockStream
}

func (m *mockHTTP3StreamForTest) CancelRead(code quic.StreamErrorCode) {
	// HTTP/3 specific cancel read
}

func (m *mockHTTP3StreamForTest) CancelWrite(code quic.StreamErrorCode) {
	// HTTP/3 specific cancel write
}

func TestHTTP3Stream(t *testing.T) {
	// Test that HTTP3Stream properly wraps the HTTP/3 stream
	mockHTTP3Stream := &mockHTTP3StreamForTest{
		mockStream: &mockStream{},
	}
	stream := NewHTTP3Stream(mockHTTP3Stream)

	// Test that it implements the Stream interface
	var _ Stream = stream

	// Test basic operations
	require.NotNil(t, stream.Context())
	
	// Test datagram operations
	testData := []byte("test")
	err := stream.SendDatagram(testData)
	require.NoError(t, err)

	// Test cancel read
	stream.CancelRead(123)
}

func TestDialHTTP2(t *testing.T) {
	// This is a basic test to ensure the DialHTTP2 function exists and has the right signature
	// A full integration test would require setting up an actual HTTP/2 server
	template := uritemplate.MustNew("https://example.com/connect-ip")
	
	// Create a mock client that will fail (since we don't have a real server)
	client := &http.Client{
		Timeout: 10 * time.Millisecond,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	
	_, _, err := DialHTTP2(ctx, client, template, "HTTP/2.0", http.Header{}, true)
	require.Error(t, err) // Expected to fail due to no server
	
	// The important thing is that the function exists and accepts the right parameters
}

func TestDialAuto(t *testing.T) {
	// Test the auto-dial function
	template := uritemplate.MustNew("https://example.com/connect-ip")
	
	// Create a mock transport
	transport := &http.Transport{}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	
	_, _, err := DialAuto(ctx, transport, template, "HTTP/2.0", http.Header{}, true)
	require.Error(t, err) // Expected to fail due to no server
	
	// The important thing is that the function exists and routes to HTTP/2
}

func TestStreamInterface(t *testing.T) {
	// Test that both HTTP2Stream and HTTP3Stream implement the Stream interface
	mockHTTP2Stream := newMockHTTP2Stream()
	http2Stream := NewHTTP2Stream(mockHTTP2Stream, context.Background())
	
	mockHTTP3Stream := &mockHTTP3StreamForTest{
		mockStream: &mockStream{},
	}
	http3Stream := NewHTTP3Stream(mockHTTP3Stream)
	
	// Verify they both implement Stream
	var _ Stream = http2Stream
	var _ Stream = http3Stream
	
	// Test that they have all required methods
	require.NotNil(t, http2Stream.Context())
	require.NotNil(t, http3Stream.Context())
	
	require.Nil(t, http2Stream.SetDeadline(time.Time{}))
	require.Nil(t, http3Stream.SetDeadline(time.Time{}))
	
	http2Stream.CancelRead(0)
	http3Stream.CancelRead(0)
	
	http2Stream.Close()
	http3Stream.Close()
}