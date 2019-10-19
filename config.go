package main

import (
	"net/http"
	"strings"
)

type config struct {
	TLS     bool   `json:"tls-active"`
	RootURL string `json:"root-url"`
	RootDIR string `json:"root-dir"`
	APIKeys struct {
		ReadOnly  []string `json:"read-only"`
		ReadWrite []string `json:"read-write"`
	} `json:"api-keys"`
}

func (c *config) baseURL(r *http.Request) string {
	var sb strings.Builder
	sb.WriteString("http")
	if c.TLS {
		sb.WriteString("s")
	}
	sb.WriteString("//")
	sb.WriteString(r.URL.Hostname())
	sb.WriteString(c.RootURL)
	return sb.String()
}

func (c *config) verifyUserOpenMode() bool {
	if len(c.APIKeys.ReadOnly) == 0 && len(c.APIKeys.ReadWrite) == 0 {
		return true
	}
	return false
}

func (c *config) verifyUserCanWrite(k string) bool {
	// Shortcut if no api-keys are present
	if c.verifyUserOpenMode() {
		return true
	}
	for _, x := range c.APIKeys.ReadWrite {
		if x == k {
			return true
		}
	}
	return false
}

func (c *config) verifyUserCanRead(k string) bool {
	if c.verifyUserCanWrite(k) {
		return true
	}
	for _, x := range c.APIKeys.ReadOnly {
		if x == k {
			return true
		}
	}
	return false
}
