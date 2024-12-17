package ws

import (
	"io"
)

const MAX_BUF_SIZE = 1024 * 1024 * 8

type wsTransport struct {
	conn          WSConn
	readingBuf    []byte
	readingBufLen int
}

func (t *wsTransport) Close() error {
	return t.conn.Close()
}

func (t *wsTransport) Write(data []byte) (int, error) {
	f := &WSFrame{
		Fin:  true,
		Op:   2, // binary frame.
		Data: data,
	}
	return t.conn.WriteFrame(f)
}

func (t *wsTransport) Read(buf []byte) (int, error) {
	if t.readingBuf == nil {
		t.readingBuf = make([]byte, 1024*4)
	}
	if t.readingBufLen > 0 {
		size := len(buf)
		if size > t.readingBufLen {
			size = t.readingBufLen
		}
		copy(buf[:size], t.readingBuf[:size])
		restLen := t.readingBufLen - size
		copy(t.readingBuf[:restLen], t.readingBuf[size:t.readingBufLen])
		t.readingBufLen = restLen
		return size, nil
	}
	for {
		f, err := t.conn.ReadFrame()
		if err != nil {
			return 0, err
		}
		if f.Op != 2 {
			// ignore non-binary frames.
			continue
		}
		if len(f.Data) <= 0 {
			continue
		}
		size := len(buf)
		if len(f.Data) < size {
			size = len(f.Data)
		}
		copy(buf[:size], f.Data[:size])
		if size < len(f.Data) {
			restLen := len(f.Data) - size
			sizeAfter := t.readingBufLen + restLen
			if sizeAfter > cap(t.readingBuf) {
				sizeAfterExtended := len(t.readingBuf) * 2
				if sizeAfterExtended > MAX_BUF_SIZE {
					return 0, io.ErrShortBuffer
				}
				bufNew := make([]byte, sizeAfterExtended)
				copy(bufNew, t.readingBuf[:t.readingBufLen])
				t.readingBuf = bufNew
			}
			copy(t.readingBuf[t.readingBufLen:sizeAfter], f.Data[size:])
			t.readingBufLen = sizeAfter
		}
		return size, nil
	}
}

// MakeTransport wraps a websocket connection to a io.ReadWriteCloser by making use of only binary type frames.
func MakeTransport(conn WSConn) io.ReadWriteCloser {
	return &wsTransport{conn: conn}
}
