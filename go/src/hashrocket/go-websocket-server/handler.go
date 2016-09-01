package main

import (
	"io"
	"log"
	"sync"

	"golang.org/x/net/websocket"
)

type benchHandler struct {
	mutex sync.RWMutex
	conns map[*websocket.Conn]struct{}
}

type WsMsg struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	respC chan bool
}

type BroadcastResult struct {
	Type          string      `json:"type"`
	Payload       interface{} `json:"payload"`
	ListenerCount int         `json:"listenerCount"`
}

func NewBenchHandler() *benchHandler {
	return &benchHandler{
		conns: make(map[*websocket.Conn]struct{}),
	}
}

func (h *benchHandler) Accept(ws *websocket.Conn) {
	send := make(chan *WsMsg, 1)
	done := make(chan struct{})
	defer func() {
		h.mutex.Lock()
		delete(h.conns, ws)
		h.mutex.Unlock()
		close(done)
		close(send)
		ws.Close()
	}

	h.mutex.Lock()
	h.conns[ws] = struct{}{}
	h.mutex.Unlock()
	go func() {
		for {
			select {
			case <-done:
				return
			case msg := <-send:
				err := websocket.JSON.Send(c, msg)
				msg.respC <- err == nil
			}
		}
	}

	for {
		var msg WsMsg
		if err := websocket.JSON.Receive(ws, &msg); err == io.EOF {
			return
		} else if err != nil {
			log.Println("websocket.JSON.Receive err:", err)
			return
		}

		switch msg.Type {
		case "echo":
			if err := h.echo(ws, msg.Payload); err != nil {
				log.Println("echo err:", err)
				return
			}
		case "broadcast":
			if err := h.broadcast(ws, msg.Payload); err != nil {
				log.Println("broadcast err:", err)
				return
			}
		default:
			log.Println("unknown msg.Type")
			return
		}
	}
}

func (h *benchHandler) echo(ws *websocket.Conn, payload interface{}) error {
	return websocket.JSON.Send(ws, &WsMsg{Type: "echo", Payload: payload})
}

func (h *benchHandler) broadcast(ws *websocket.Conn, payload interface{}) error {
	result := BroadcastResult{Type: "broadcastResult", Payload: payload}
	request := &WsMsg{Type: "broadcast", Payload: payload}

	h.mutex.RLock()

	for c := range h.conns {
		if err := websocket.JSON.Send(c, request); err == nil {
			result.ListenerCount++
		}
	}

	h.mutex.RUnlock()

	return websocket.JSON.Send(ws, &result)
}

func (h *benchHandler) cleanup(ws *websocket.Conn) {
}
