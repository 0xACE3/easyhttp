package easyhttp

import (
	"math/rand"
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
