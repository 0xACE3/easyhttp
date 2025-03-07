package easyhttp

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"
)

type BrowserType string

const (
	Chrome  BrowserType = "chrome"
	Safari  BrowserType = "safari"
	Firefox BrowserType = "firefox"
)

func GetRandomBrowserType() BrowserType {
	browsers := []BrowserType{Chrome, Safari, Firefox}
	randomIndex := rand.Intn(len(browsers))
	return browsers[randomIndex]
}

func NormalizeURL(url string) string {
	url = strings.TrimRight(url, "/")
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	return url
}

func BuildQueryString(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}

	values := url.Values{}

	for key, value := range params {
		switch v := value.(type) {
		case []string:
			for _, item := range v {
				values.Add(key+"[]", item)
			}
		default:
			values.Add(key, fmt.Sprint(v))
		}
	}

	return values.Encode()
}

func toHttpHeaders(headers map[string]string) map[string][]string {
	converted := make(map[string][]string)
	for k, v := range headers {
		converted[k] = []string{v}
	}
	return converted
}
