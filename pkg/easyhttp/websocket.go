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
	defer c.mutx.Unlock()

	if ws, exists := c.wsConns[wsUrl]; exists {
		if !ws.closed {
			return ws, nil
		}
		delete(c.wsConns, wsUrl)
	}

	dialer := websocket.Dialer{
		EnableCompression: c.config.WSConfig.EnableCompression,
	}

	if c.config.Proxy != "" {
		proxyURL, err := url.Parse(c.config.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %v", err)
		}
		dialer.Proxy = http.ProxyURL(proxyURL)
	}

	conn, _, err := dialer.DialContext(ctx, wsUrl, convertHeaders(headers))
	if err != nil {
		return nil, fmt.Errorf("websocket connection failed: %v", err)
	}

	ws := &WebSocketClient{
		conn:     conn,
		messages: make(chan []byte, c.config.WSConfig.BufferSize),
		done:     make(chan struct{}),
	}

	ws.wg.Add(2)
	go ws.readLoop(c.config.IsDebug, c.config.WSConfig)
	go ws.pingLoop(c.config.IsDebug, c.config.WSConfig)

	c.wsConns[wsUrl] = ws
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
			_, message, err := ws.conn.ReadMessage()
			if err != nil {
				if debug {
					log.Printf("websocket read error: %v", err)
				}
				return
			}

			select {
			case ws.messages <- message:
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
			ws.mutx.Lock()
			if ws.closed {
				ws.mutx.RUnlock()
				return
			}
			err := ws.conn.WriteMessage(websocket.PingMessage, nil)
			ws.mutx.Unlock()

			if err != nil {
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

func (w *WebSocketClient) Messages() <-chan []byte {
	return w.messages
}

func (ws *WebSocketClient) Send(msg interface{}) error {
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
	ws.mutx.Unlock()

	ws.wg.Wait()

	ws.mutx.Lock()
	if ws.messages != nil {
		close(ws.messages)
		ws.messages = nil
	}
	err := ws.conn.Close()
	ws.mutx.Unlock()

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