package proxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && headerContainsToken(r.Header.Get("Connection"), "upgrade")
}

func headerContainsToken(value, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func (h *Handler) proxyWebSocket(id uint64, w http.ResponseWriter, r *http.Request, route Route, body []byte) error {
	if len(body) != 0 {
		return fmt.Errorf("websocket request unexpectedly had body bytes=%d", len(body))
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket hijacking not supported", http.StatusInternalServerError)
		return fmt.Errorf("response writer does not support hijacking")
	}

	upstreamConn, err := tls.DialWithDialer(&net.Dialer{}, "tcp", route.Upstream+":443", &tls.Config{ServerName: route.Upstream, MinVersion: tls.VersionTLS12})
	if err != nil {
		http.Error(w, "upstream websocket dial failed", http.StatusBadGateway)
		return err
	}
	defer upstreamConn.Close()

	upstreamReq := r.Clone(r.Context())
	upstreamReq.URL = &url.URL{
		Scheme:     "https",
		Host:       route.Upstream,
		Path:       r.URL.Path,
		RawPath:    r.URL.RawPath,
		RawQuery:   r.URL.RawQuery,
		ForceQuery: r.URL.ForceQuery,
	}
	upstreamReq.RequestURI = ""
	upstreamReq.Host = route.Upstream
	upstreamReq.Header = cloneHeaders(r.Header)
	upstreamReq.Body = http.NoBody
	upstreamReq.ContentLength = 0

	if err := upstreamReq.Write(upstreamConn); err != nil {
		http.Error(w, "upstream websocket write failed", http.StatusBadGateway)
		return err
	}

	br := bufio.NewReader(upstreamConn)
	upstreamResp, err := http.ReadResponse(br, upstreamReq)
	if err != nil {
		http.Error(w, "upstream websocket response failed", http.StatusBadGateway)
		return err
	}
	defer upstreamResp.Body.Close()

	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		return err
	}
	defer clientConn.Close()

	if clientBuf.Reader.Buffered() != 0 {
		h.log.Info("id=%d buffered_client_bytes=%d\n", id, clientBuf.Reader.Buffered())
	}

	if err := upstreamResp.Write(clientConn); err != nil {
		return err
	}
	h.log.Websocket("id=%d status=%d content_type=%q\n", id, upstreamResp.StatusCode, upstreamResp.Header.Get("Content-Type"))
	if upstreamResp.StatusCode != http.StatusSwitchingProtocols {
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(upstreamConn, clientConn)
		_ = upstreamConn.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, br)
		_ = clientConn.Close()
	}()
	wg.Wait()
	h.log.Websocket("id=%d complete=true\n", id)
	return nil
}

func cloneHeaders(headers http.Header) http.Header {
	out := make(http.Header, len(headers))
	for name, values := range headers {
		out[name] = append([]string(nil), values...)
	}
	return out
}
