package proxy

import "unicode/utf8"

type ResponsePreview struct {
	limit int
	buf   []byte
}

func NewResponsePreview(limit int) *ResponsePreview {
	if limit < 0 {
		limit = 0
	}
	return &ResponsePreview{limit: limit}
}

func (p *ResponsePreview) Observe(chunk []byte) {
	if p == nil || p.limit == 0 || len(p.buf) >= p.limit {
		return
	}
	remaining := p.limit - len(p.buf)
	if len(chunk) > remaining {
		chunk = chunk[:remaining]
	}
	p.buf = append(p.buf, chunk...)
}

func (p *ResponsePreview) String() string {
	if p == nil || len(p.buf) == 0 {
		return ""
	}
	buf := p.buf
	for len(buf) > 0 && !utf8.Valid(buf) {
		buf = buf[:len(buf)-1]
	}
	return string(buf)
}
