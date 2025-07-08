# Proxying IP over HTTP

Quick and dirty fork of the awesome [connect-ip-go](https://github.com/quic-go/connect-ip-go) to add support for the non [RFC 9484](https://datatracker.ietf.org/doc/html/rfc9484) compliant Cloudflare implementation.

Unless you have very specific needs like me, you should probably use the original. There is no guarantee for API stability or feature parity with the original library.

## New Features

### HTTP/2 Support
This fork now supports both HTTP/2 and HTTP/3 for MASQUE connections:

- **HTTP/3**: Original implementation using QUIC datagrams (existing functionality)
- **HTTP/2**: New implementation using framed data over HTTP/2 streams
- **Auto-detection**: Automatically chooses the appropriate protocol based on the transport

### Usage Examples

#### HTTP/3 (Original)
```go
// HTTP/3 with QUIC datagrams
conn, resp, err := connectip.Dial(ctx, http3ClientConn, template, "HTTP/3.0", headers, false)
```

#### HTTP/2 (New)
```go
// HTTP/2 with framed data over streams
client := &http.Client{Transport: &http2.Transport{...}}
conn, resp, err := connectip.DialHTTP2(ctx, client, template, "HTTP/2.0", headers, false)
```

#### Auto-detection (New)
```go
// Automatically detects protocol based on transport
conn, resp, err := connectip.DialAuto(ctx, transport, template, "HTTP/2.0", headers, false)
```

See the [examples/](examples/) directory for complete examples of both HTTP/2 and HTTP/3 usage.
