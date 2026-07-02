package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

func readAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil || r.Body == http.NoBody {
		r.Body = http.NoBody
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if closeErr := r.Body.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func MakeUpstreamRequest(in *http.Request, route Route, body []byte) (*http.Request, error) {
	if route.Upstream == "" {
		return nil, fmt.Errorf("route has no upstream")
	}
	outURL := &url.URL{
		Scheme:     "https",
		Host:       route.Upstream,
		Path:       in.URL.Path,
		RawPath:    in.URL.RawPath,
		RawQuery:   in.URL.RawQuery,
		ForceQuery: in.URL.ForceQuery,
	}

	out, err := http.NewRequestWithContext(in.Context(), in.Method, outURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	out.Header = StripHopByHopHeaders(in.Header)
	out.Header.Set("Accept-Encoding", "identity")
	out.Host = route.Upstream
	out.ContentLength = int64(len(body))
	if len(body) == 0 {
		out.Body = http.NoBody
	}
	return out, nil
}

func StripHopByHopHeaders(headers http.Header) http.Header {
	connectionTokens := map[string]struct{}{}
	for _, value := range headers.Values("Connection") {
		for _, rawToken := range strings.Split(value, ",") {
			token := strings.ToLower(strings.TrimSpace(rawToken))
			if token != "" {
				connectionTokens[token] = struct{}{}
			}
		}
	}

	out := make(http.Header, len(headers))
	for name, values := range headers {
		lower := strings.ToLower(name)
		if _, ok := hopByHopHeaders[lower]; ok {
			continue
		}
		if _, ok := connectionTokens[lower]; ok {
			continue
		}
		out[name] = append([]string(nil), values...)
	}
	return out
}

var hopByHopHeaders = map[string]struct{}{
	"connection":          {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"proxy-connection":    {},
	"te":                  {},
	"trailer":             {},
	"transfer-encoding":   {},
	"upgrade":             {},
}

func copyHeaders(dst, src http.Header) {
	keys := make([]string, 0, len(src))
	for key := range src {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range src[key] {
			dst.Add(key, value)
		}
	}
}

func streamResponse(w http.ResponseWriter, body io.Reader, observer *SSEObserver, preview *ResponsePreview) (int64, error) {
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	var total int64
	defer func() {
		if observer != nil {
			observer.Finish()
		}
	}()
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if observer != nil {
				observer.Observe(chunk)
			}
			if preview != nil {
				preview.Observe(chunk)
			}
			written, writeErr := w.Write(chunk)
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return total, nil
			}
			return total, readErr
		}
	}
}
