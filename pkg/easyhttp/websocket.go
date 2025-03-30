package easyhttp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func (c *ApiClient) newWebSocket(ctx context.Context, wsUrl string, headers map[string]string) (*WebSocketClient, error) {
	c.mutx.Lock()

	if ws, exists := c.wsConns[wsUrl]; exists && !ws.closed {
		c.mutx.Unlock()
		return ws, nil
	}
	c.mutx.Unlock()

	dialer := &websocket.Dialer{
		EnableCompression: c.config.WSConfig.EnableCompression,
	}

	if c.config.WsProxy != "" {
		proxyURL, err := url.Parse(c.config.WsProxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %v", err)
		}
		dialer.Proxy = http.ProxyURL(proxyURL)
	}

	conn, resp, err := dialer.DialContext(ctx, wsUrl, toHttpHeaders(headers))
	if err != nil {
		return nil, fmt.Errorf("websocket dial failed: %v", err)
	}

	ws := &WebSocketClient{
		conn:        conn,
		messages:    make(chan []byte, c.config.WSConfig.BufferSize),
		done:        make(chan struct{}),
		RespHeaders: resp.Header,
	}

	ws.wg.Add(2)
	go ws.readLoop(c.config.IsDebug, c.config.WSConfig)
	go ws.pingLoop(c.config.IsDebug, c.config.WSConfig)

	c.mutx.Lock()
	c.wsConns[wsUrl] = ws
	c.mutx.Unlock()
	return ws, nil
}

func (ws *WebSocketClient) readLoop(debug bool, config *WebSocketConfig) {
	defer ws.wg.Done()
	defer ws.Close()

	ws.conn.SetReadDeadline(time.Now().Add(config.PongWait))
	ws.conn.SetPongHandler(func(string) error {
		return ws.conn.SetReadDeadline(time.Now().Add(config.PongWait))
	})

	for {
		select {
		case <-ws.done:
			return
		default:
			_, msg, err := ws.conn.ReadMessage()
			if err != nil {
				if debug && !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					log.Printf("websocket read error: %v", err)
				}
				return
			}
			select {
			case ws.messages <- msg:
			case <-ws.done:
				return
			default:
				if debug {
					log.Println("message buffer full, dropping message")
				}
			}
		}
	}
}

func (ws *WebSocketClient) pingLoop(debug bool, config *WebSocketConfig) {
	defer ws.wg.Done()

	ticker := time.NewTicker(config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := ws.sendPing(); err != nil {
				if debug {
					log.Printf("ping error: %v", err)
				}
				return
			}
		case <-ws.done:
			return
		}
	}
}

func (ws *WebSocketClient) sendPing() error {
	ws.mutx.Lock()
	defer ws.mutx.Unlock()
	if ws.closed {
		return nil
	}
	return ws.conn.WriteMessage(websocket.PingMessage, nil)
}

func (w *WebSocketClient) Messages() <-chan []byte {
	return w.messages
}

func (ws *WebSocketClient) Send(msg any) error {
	ws.mutx.Lock()
	defer ws.mutx.Unlock()

	if ws.closed {
		return fmt.Errorf("websocket connection is closed")
	}

	return ws.conn.WriteJSON(msg)
}

func (ws *WebSocketClient) Close() error {
	ws.mutx.Lock()
	if ws.closed {
		ws.mutx.Unlock()
		return nil
	}
	ws.closed = true
	close(ws.done)
	if ws.messages != nil {
		close(ws.messages)
		ws.messages = nil
	}
	ws.mutx.Unlock()

	err := ws.conn.Close()
	ws.wg.Wait()
	return err
}

func (c *ApiClient) RemoveClosedWebSockets() {
	c.mutx.Lock()
	defer c.mutx.Unlock()

	for url, ws := range c.wsConns {
		if ws.closed {
			delete(c.wsConns, url)
		}
	}
}

func (c *ApiClient) CloseAllWebSockets() {
	c.mutx.Lock()
	conns := make([]*WebSocketClient, 0, len(c.wsConns))
	for _, ws := range c.wsConns {
		if !ws.closed {
			conns = append(conns, ws)
		}
	}
	c.wsConns = make(map[string]*WebSocketClient)
	c.mutx.Unlock()

	for _, ws := range conns {
		if err := ws.Close(); err != nil && c.config.IsDebug {
			log.Printf("error closing websocket: %v", err)
		}
	}
}
