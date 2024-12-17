package ws

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

var UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// Dial is to initiate a websocket connection.
// Dial accepts an URL argument like `wss://localhost:3243/hi`.
func Dial(uriStr string) (WSConn, error) {
	uri, err := url.Parse(uriStr)
	if err != nil {
		return nil, err
	}

	scheme := uri.Scheme
	if len(scheme) == 0 {
		scheme = "wss"
	}
	if !strings.EqualFold(scheme, "wss") && !strings.EqualFold(scheme, "ws") {
		return nil, fmt.Errorf(
			"Invalid uri scheme `%s`. Only ws/wss supported.",
			scheme,
		)
	}

	port := 443
	useTLS := true

	if strings.EqualFold(scheme, "ws") {
		port = 80
		useTLS = false
	}

	host := uri.Host
	if h, portStr, err := net.SplitHostPort(host); err == nil {
		host = h
		if portVal, err := strconv.Atoi(portStr); err == nil {
			port = portVal
		}
	}

	// connect to server
	hostAddr := fmt.Sprintf("%s:%d", host, port)
	conn := (io.ReadWriteCloser)(nil)
	if useTLS {
		cfg := &tls.Config{
			InsecureSkipVerify: true,
		}
		if connTls, err := tls.Dial("tcp", hostAddr, cfg); err != nil {
			return nil, fmt.Errorf("tls.Dial(%s): %v", hostAddr, err)
		} else {
			conn = connTls
		}
	} else {
		if connTcp, err := net.Dial("tcp", hostAddr); err != nil {
			return nil, fmt.Errorf("net.Dial(tcp,%s): %v", hostAddr, err)
		} else {
			conn = connTcp
		}
	}

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	tx := bufio.NewReadWriter(r, w)

	// handshaking
	hsKeyb := make([]byte, 8)
	rand.Read(hsKeyb)
	hsKey := base64.StdEncoding.EncodeToString(hsKeyb)
	hsAckStr := fmt.Sprintf("%s%s", hsKey, wsGUID)
	hsAckb := sha1.Sum([]byte(hsAckStr))
	hsAccept := base64.StdEncoding.EncodeToString(hsAckb[:])

	// sending request
	fmt.Fprintf(tx, "GET %s HTTP/1.1\r\n", uri.String())
	fmt.Fprintf(tx, "Host: %s\r\n", uri.Host)
	fmt.Fprintf(tx, "User-Agent: %s\r\n", UserAgent)
	fmt.Fprintf(tx, "Connection: Upgrade\r\n")
	fmt.Fprintf(tx, "Upgrade: websocket\r\n")
	fmt.Fprintf(tx, "Sec-WebSocket-Version: 7\r\n")
	fmt.Fprintf(tx, "Sec-WebSocket-Key: %s\r\n", hsKey)
	fmt.Fprintf(tx, "\r\n")
	tx.Flush()

	headerScanner := bufio.NewScanner(tx)
	eoh := []byte("\r\n\r\n")
	headerScanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.Index(data, eoh); i >= 0 {
			chunk := make([]byte, i)
			copy(chunk, data[:i])
			return i + len(eoh), chunk, bufio.ErrFinalToken
		}
		if atEOF {
			return len(data), data, bufio.ErrFinalToken
		}
		return 0, nil, nil
	})
	if !headerScanner.Scan() {
		return nil, fmt.Errorf("Invalid HTTP request.")
	}
	headerb := headerScanner.Bytes()
	headerSrc := bytes.NewReader(headerb)
	headerR := bufio.NewReader(headerSrc)
	resp, err := parseHttpHeader(headerR)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("parseHttpHeader: %v", err)
	}
	if resp.StatusCode != 101 {
		return nil, fmt.Errorf("status %s", resp.Status)
	}

	errHsFail := fmt.Errorf("Handshaking failed.")
	if !strings.EqualFold("upgrade", resp.Header.Get("Connection")) {
		return nil, errHsFail
	}
	if !strings.EqualFold("websocket", resp.Header.Get("Upgrade")) {
		return nil, errHsFail
	}
	if !strings.EqualFold(hsAccept, resp.Header.Get("Sec-WebSocket-Accept")) {
		return nil, errHsFail
	}

	lckR := new(sync.Mutex)
	lckW := new(sync.Mutex)
	return &wsConn{closer: conn, tx: tx, lckR: lckR, lckW: lckW, mask: true}, nil
}
