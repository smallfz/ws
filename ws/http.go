package ws

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
)

func badStringError(t, t1 string) error {
	return fmt.Errorf("%s %s", t, t1)
}

// parseHttpHeader copies a lot of codes from net/http codebase.
func parseHttpHeader(r *bufio.Reader) (*http.Response, error) {
	tp := textproto.NewReader(r)
	resp := &http.Response{}

	// Parse the first line of the response.
	line, err := tp.ReadLine()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	proto, status, ok := strings.Cut(line, " ")
	if !ok {
		return nil, badStringError("malformed HTTP response", line)
	}
	resp.Proto = proto
	resp.Status = strings.TrimLeft(status, " ")

	statusCode, _, _ := strings.Cut(resp.Status, " ")
	if len(statusCode) != 3 {
		return nil, badStringError("malformed HTTP status code", statusCode)
	}
	resp.StatusCode, err = strconv.Atoi(statusCode)
	if err != nil || resp.StatusCode < 0 {
		return nil, badStringError("malformed HTTP status code", statusCode)
	}
	if resp.ProtoMajor, resp.ProtoMinor, ok = http.ParseHTTPVersion(resp.Proto); !ok {
		return nil, badStringError("malformed HTTP version", resp.Proto)
	}

	// Parse the response headers.
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil && err != io.EOF {
		return nil, err
	}
	resp.Header = http.Header(mimeHeader)

	return resp, nil
}
