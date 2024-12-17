package ws

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// WebSocketHandshake works at server-side. It hijacks the connection and continues with handshaking, after that it returns a closable frame read-writer.
func WebSocketHandshake(req *http.Request, w http.ResponseWriter) (WSConn, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("Server doesn't support hijacking.")
	}

	conn, tx, err := hj.Hijack()
	if err != nil {
		return nil, fmt.Errorf("Hijack(): %v", err)
	}

	proto := req.Proto
	header := req.Header

	isWs := false
	wsAccept := ""
	if strings.EqualFold(header.Get("Upgrade"), "websocket") {
		isWs = true
		wsKey := header.Get("Sec-WebSocket-Key")
		if len(wsKey) > 0 {
			wsAckStr := fmt.Sprintf("%s%s", wsKey, wsGUID)
			wsAckb := sha1.Sum([]byte(wsAckStr))
			wsAccept = base64.StdEncoding.EncodeToString(wsAckb[:])
		}
	}

	if !isWs {
		defer conn.Close()
		return nil, fmt.Errorf("Not a websocket request.")
	}

	// sending response
	wsProto := header.Get("Sec-WebSocket-Protocol")
	fmt.Fprintf(tx, "%s 101 Switching Protocols\r\n", proto)
	fmt.Fprintf(tx, "Connection: Upgrade\r\n")
	fmt.Fprintf(tx, "Upgrade: websocket\r\n")
	fmt.Fprintf(tx, "Sec-WebSocket-Accept: %s\r\n", wsAccept)
	if len(wsProto) > 0 {
		fmt.Fprintf(tx, "Sec-WebSocket-Protocol: %s\r\n", wsProto)
	}
	fmt.Fprintf(tx, "Sec-WebSocket-Version: 7\r\n")
	fmt.Fprintf(tx, "\r\n")
	tx.Flush()

	lckR := new(sync.Mutex)
	lckW := new(sync.Mutex)
	return &wsConn{closer: conn, tx: tx, lckR: lckR, lckW: lckW}, nil
}
