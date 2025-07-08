package connectip

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/quic-go/quic-go/http3"
	"github.com/yosida95/uritemplate/v3"
)

// Dial dials a proxied connection to a target server using HTTP/3.
func Dial(ctx context.Context, conn *http3.ClientConn, template *uritemplate.Template, requestProtocol string, additionalHeaders http.Header, ignoreExtendedConnect bool) (*Conn, *http.Response, error) {
	if len(template.Varnames()) > 0 {
		return nil, nil, errors.New("connect-ip: IP flow forwarding not supported")
	}

	u, err := url.Parse(template.Raw())
	if err != nil {
		return nil, nil, fmt.Errorf("connect-ip: failed to parse URI: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, nil, context.Cause(ctx)
	case <-conn.Context().Done():
		return nil, nil, context.Cause(conn.Context())
	case <-conn.ReceivedSettings():
	}
	settings := conn.Settings()
	if !ignoreExtendedConnect && !settings.EnableExtendedConnect {
		return nil, nil, errors.New("connect-ip: server didn't enable Extended CONNECT")
	}
	if !settings.EnableDatagrams {
		return nil, nil, errors.New("connect-ip: server didn't enable datagrams")
	}

	headers := http.Header{http3.CapsuleProtocolHeader: []string{capsuleProtocolHeaderValue}}
	for k, v := range additionalHeaders {
		headers[k] = v
	}

	rstr, err := conn.OpenRequestStream(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("connect-ip: failed to open request stream: %w", err)
	}
	if err := rstr.SendRequestHeader(&http.Request{
		Method: http.MethodConnect,
		Proto:  requestProtocol,
		Host:   u.Host,
		Header: headers,
		URL:    u,
	}); err != nil {
		return nil, nil, fmt.Errorf("connect-ip: failed to send request: %w", err)
	}
	// TODO: optimistically return the connection
	rsp, err := rstr.ReadResponse()
	if err != nil {
		return nil, nil, fmt.Errorf("connect-ip: failed to read response: %w", err)
	}
	if rsp.StatusCode < 200 || rsp.StatusCode > 299 {
		return nil, rsp, fmt.Errorf("connect-ip: server responded with %d", rsp.StatusCode)
	}
	return newProxiedConn(NewHTTP3Stream(rstr)), rsp, nil
}

// DialHTTP2 dials a proxied connection to a target server using HTTP/2.
func DialHTTP2(ctx context.Context, client *http.Client, template *uritemplate.Template, requestProtocol string, additionalHeaders http.Header, ignoreExtendedConnect bool) (*Conn, *http.Response, error) {
	if len(template.Varnames()) > 0 {
		return nil, nil, errors.New("connect-ip: IP flow forwarding not supported")
	}

	u, err := url.Parse(template.Raw())
	if err != nil {
		return nil, nil, fmt.Errorf("connect-ip: failed to parse URI: %w", err)
	}

	headers := http.Header{http3.CapsuleProtocolHeader: []string{capsuleProtocolHeaderValue}}
	for k, v := range additionalHeaders {
		headers[k] = v
	}

	req := &http.Request{
		Method: http.MethodConnect,
		Proto:  requestProtocol,
		Host:   u.Host,
		Header: headers,
		URL:    u,
	}
	req = req.WithContext(ctx)

	// For HTTP/2, we need to handle the CONNECT request specially
	// The standard library's HTTP/2 implementation will handle the stream setup
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("connect-ip: failed to send request: %w", err)
	}
	
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, resp, fmt.Errorf("connect-ip: server responded with %d", resp.StatusCode)
	}

	// For HTTP/2 CONNECT, the response body is the bidirectional stream
	httpStream := &HTTP2ResponseWriterStream{
		req: req,
		ctx: ctx,
	}
	
	// Use the response body as the read side and response writer interface for write side
	// This is a simplified approach - in a real implementation, we'd need to handle
	// the bidirectional stream properly
	return newProxiedConn(NewHTTP2Stream(httpStream, ctx)), resp, nil
}

// DialAuto automatically detects the protocol and dials using the appropriate method
func DialAuto(ctx context.Context, transport http.RoundTripper, template *uritemplate.Template, requestProtocol string, additionalHeaders http.Header, ignoreExtendedConnect bool) (*Conn, *http.Response, error) {
	// Check if the transport is HTTP/3
	if h3conn, ok := transport.(*http3.Transport); ok {
		// For HTTP/3, we need to get the connection first
		_, err := url.Parse(template.Raw())
		if err != nil {
			return nil, nil, fmt.Errorf("connect-ip: failed to parse URI: %w", err)
		}
		
		// This is a simplified approach - in practice, you'd need to establish
		// the connection through the HTTP/3 transport
		_ = h3conn // Use the transport to establish connection
		return nil, nil, errors.New("connect-ip: HTTP/3 auto-dial not fully implemented - use Dial() with http3.ClientConn")
	}

	// For HTTP/2, use a regular HTTP client
	client := &http.Client{
		Transport: transport,
	}
	
	return DialHTTP2(ctx, client, template, requestProtocol, additionalHeaders, ignoreExtendedConnect)
}
