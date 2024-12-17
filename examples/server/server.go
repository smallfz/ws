package main

import (
	"github.com/smallfz/ws/ws"
	"io"
	"net/http"
)

func main() {
	echo := func(w http.ResponseWriter, req *http.Request) {
		conn, err := ws.WebSocketHandshake(req, w)
		if err != nil {
			http.Error(w, "handshake fail.", 400)
			return
		}
		defer conn.Close()

		ch := make(chan *ws.WSFrame)

		go func() {
			defer func() {
				ch <- nil
			}()
			for {
				if f, err := conn.ReadFrame(); err != nil {
					if err != io.EOF {
						println(err.Error())
					}
					return
				} else {
					ch <- f
				}
			}
		}()

		x := req.Context()
		for {
			select {
			case <-x.Done():
				return
			case frame := <-ch:
				if frame == nil {
					return
				}
				msg := string(frame.Data)
				println(msg)
				reply := &ws.WSFrame{
					Fin:  true,
					Op:   1, // text frame
					Data: []byte(msg),
				}
				if _, err := conn.WriteFrame(reply); err != nil {
					return
				}
			}
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/echo", echo)

	http.ListenAndServe(":8080", mux)
}
