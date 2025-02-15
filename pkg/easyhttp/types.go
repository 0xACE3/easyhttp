package easyhttp

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/imroc/req/v3"
	"golang.org/x/sync/semaphore"
)

type ClientConfig struct {
	Timeout        time.Duration
	MaxConcurrency int64
	Proxy          string
	RetryCount     int
	RetryWait      time.Duration
	RetryMaxWait   time.Duration
	BrowserType    BrowserType
	IsDebug        bool
	WSConfig       *WebSocketConfig
}

type WebSocketConfig struct {
	EnableCompression bool
	PingInterval      time.Duration
	PongWait          time.Duration
	BufferSize        int
}

type WebSocketClient struct {
	conn     *websocket.Conn
	messages chan []byte
	done     chan struct{}
	closed   bool
	mutx     sync.RWMutex
	wg       sync.WaitGroup
}

type ApiClient struct {
	baseUrl string
	client  *req.Client
	sema    *semaphore.Weighted
	mutx    sync.Mutex
	config  *ClientConfig
	wsConns map[string]*WebSocketClient
}

type APIError struct {
	Message string
	Status  int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: %s", e.Message)
}

func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Timeout:        30 * time.Second,
		MaxConcurrency: 5,
		RetryCount:     3,
		RetryWait:      2 * time.Second,
		RetryMaxWait:   5 * time.Second,
		BrowserType:    GetRandomBrowserType(),
		IsDebug:        false,
		WSConfig:       DefaultWebSocketConfig(),
	}
}

func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		EnableCompression: true,
		BufferSize:        100,
		PingInterval:      30 * time.Second,
		PongWait:          60 * time.Second,
	}
}
