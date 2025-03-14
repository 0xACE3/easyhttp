package easyhttp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/imroc/req/v3"
	"golang.org/x/sync/semaphore"
)

func NewApiClient(url string, config *ClientConfig) *ApiClient {

	if config == nil {
		config = DefaultClientConfig()
	}

	client := &ApiClient{
		baseUrl: NormalizeURL(url),
		config:  config,
		sema:    semaphore.NewWeighted(config.MaxConcurrency),
		wsConns: make(map[string]*WebSocketClient),
	}

	client.initialize()

	return client
}

func (api *ApiClient) initialize() {
	api.mutx.Lock()
	defer api.mutx.Unlock()

	if api.client != nil {
		return
	}

	client := req.C()

	switch api.config.BrowserType {
	case Chrome:
		client.ImpersonateChrome()
	case Safari:
		client.ImpersonateSafari()
	case Firefox:
		client.ImpersonateFirefox()
	default:
		client.SetTLSFingerprintRandomized()
	}

	if api.config.Proxy != "" {
		client.SetProxyURL(api.config.Proxy)
	}

	api.client = client
}

func (api *ApiClient) request(ctx context.Context, method, endpoint string, payload any, result any, headers map[string]string) error {

	if api.client == nil {
		return fmt.Errorf("client is not initialized")
	}

	url := fmt.Sprintf("%s/%s", api.baseUrl, strings.TrimLeft(endpoint, "/"))

	if err := api.sema.Acquire(ctx, 1); err != nil {
		return err
	}

	defer api.sema.Release(1)

	return api.makeRequest(ctx, method, url, payload, result, headers)

}

func (api *ApiClient) makeRequest(ctx context.Context, method, url string, payload any, result any, headers map[string]string) error {

	attempt := 0
	maxAttempts := api.config.RetryCount

	request := api.client.R().
		SetContext(ctx).
		SetRetryCount(api.config.RetryCount).
		SetRetryBackoffInterval(api.config.RetryWait, api.config.RetryMaxWait).
		SetSuccessResult(result).
		SetHeaders(headers).
		AddRetryHook(func(resp *req.Response, err error) {
			attempt++
			if api.config.IsDebug {
				log.Printf("retrying request attempt (%d/%d) - host: %s - status: %d", attempt, maxAttempts, resp.Request.URL.Host, resp.GetStatusCode())
			}
		})

	if payload != nil {
		request.SetBody(payload)
	}

	var response *req.Response
	var err error

	switch method {
	case http.MethodGet:
		response, err = request.Get(url)

	case http.MethodPost:
		response, err = request.Post(url)

	case http.MethodPut:
		response, err = request.Put(url)

	case http.MethodDelete:
		response, err = request.Delete(url)

	default:
		return fmt.Errorf("unsupported method: %s", method)
	}

	status := response.GetStatusCode()

	if err != nil {
		return &APIError{
			Message: fmt.Sprintf("request failed: %v", err),
			Status:  status,
		}
	}

	if response.IsErrorState() {
		return &APIError{
			Message: fmt.Sprintf("request failed with status: %d", status),
			Status:  status,
		}
	}
	if api.config.IsDebug {
		log.Printf("request url: %s is successful with status: %d", url, status)
	}
	return nil
}
