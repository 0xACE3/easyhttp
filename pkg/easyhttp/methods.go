package easyhttp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/imroc/req/v3"
)

func (c *ApiClient) Get(ctx context.Context, endpoint string, result interface{}, headers map[string]string) error {
	return c.request(ctx, http.MethodGet, endpoint, nil, result, headers)
}

func (c *ApiClient) Post(ctx context.Context, endpoint string, payload interface{}, result interface{}, headers map[string]string) error {
	return c.request(ctx, http.MethodPost, endpoint, payload, result, headers)
}

func (c *ApiClient) Put(ctx context.Context, endpoint string, payload interface{}, result interface{}, headers map[string]string) error {
	return c.request(ctx, http.MethodPut, endpoint, payload, result, headers)
}

func (c *ApiClient) Delete(ctx context.Context, endpoint string, result interface{}, headers map[string]string) error {
	return c.request(ctx, http.MethodDelete, endpoint, nil, result, headers)
}

func (c *ApiClient) GetRaw(ctx context.Context, endpoint string, headers map[string]string) (*req.Response, error) {
	if err := c.sema.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer c.sema.Release(1)

	url := fmt.Sprintf("%s/%s", c.baseUrl, strings.TrimLeft(endpoint, "/"))

	request := c.client.R().
		SetContext(ctx).
		SetRetryCount(c.config.RetryCount).
		SetRetryBackoffInterval(c.config.RetryWait, c.config.RetryMaxWait).
		SetHeaders(headers)

	response, err := request.Get(url)

	if err != nil {
		return nil, &APIError{
			Message: fmt.Sprintf("request failed: %v", err),
			Status:  response.GetStatusCode(),
		}
	}

	if response.IsErrorState() {
		return nil, &APIError{
			Message: fmt.Sprintf("request failed with status: %d", response.GetStatusCode()),
			Status:  response.GetStatusCode(),
		}
	}
	if c.config.IsDebug {
		log.Printf("request url: %s is successful with status: %d", url, response.GetStatusCode())
	}
	return response, nil
}
