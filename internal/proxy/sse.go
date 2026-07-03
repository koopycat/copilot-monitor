package proxy

import (
	"bytes"
	"encoding/json"
	"strings"
)

type SSEObserver struct {
	buf         []byte
	plainJSON   bool
	Bytes       int64
	Usage       Usage
	UsageSeen   bool
	Model       string
	ParseErrors int
}

func NewSSEObserver() *SSEObserver {
	return &SSEObserver{}
}

func NewJSONObserver() *SSEObserver {
	return &SSEObserver{plainJSON: true}
}

func (o *SSEObserver) Observe(chunk []byte) {
	o.Bytes += int64(len(chunk))
	o.buf = append(o.buf, chunk...)
	if o.plainJSON {
		if len(o.buf) > 4*1024*1024 {
			o.buf = nil
			o.ParseErrors++
		}
		return
	}

	for {
		idx := bytes.IndexByte(o.buf, '\n')
		if idx < 0 {
			if len(o.buf) > 1024*1024 {
				o.buf = o.buf[:0]
				o.ParseErrors++
			}
			return
		}

		line := append([]byte(nil), o.buf[:idx]...)
		o.buf = o.buf[idx+1:]
		o.processLine(line)
	}
}

func (o *SSEObserver) Finish() {
	if len(bytes.TrimSpace(o.buf)) == 0 {
		o.buf = nil
		return
	}
	if o.plainJSON {
		o.processJSON(o.buf)
		o.buf = nil
		return
	}
	line := append([]byte(nil), o.buf...)
	o.buf = nil
	o.processLine(line)
}

func (o *SSEObserver) processLine(line []byte) {
	line = bytes.TrimSuffix(line, []byte("\r"))
	trimmed := strings.TrimSpace(string(line))
	if !strings.HasPrefix(trimmed, "data:") {
		return
	}

	data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if data == "" || data == "[DONE]" {
		return
	}

	o.processJSON([]byte(data))
}

func (o *SSEObserver) processJSON(data []byte) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		o.ParseErrors++
		return
	}

	if usage, ok := findUsage(value); ok {
		o.Usage = usage
		o.UsageSeen = true
	}
	if model, ok := findStringKey(value, "model"); ok && model != "" {
		o.Model = model
	}
}
