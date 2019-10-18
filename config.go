package main

import (
	"log"
	"net/http"
	"strings"
)

type Config struct {
	TLS     bool   `json:"tls-active"`
	RootURL string `json:"root-url"`
	RootDIR string `json:"root-dir"`
	APIKeys struct {
		ReadOnly  []string `json:"read-only"`
		ReadWrite []string `json:"read-write"`
	} `json:"api-keys"`
}

func (c *Config) baseURL(r *http.Request) string {
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

func (c *Config) checkCanRead(k string) bool {
	for _, x := range c.APIKeys.ReadWrite {
		if x == k {
			return true
		}
	}
	for _, x := range c.APIKeys.ReadOnly {
		if x == k {
			return true
		}
	}
	if len(c.APIKeys.ReadOnly) == 0 && len(c.APIKeys.ReadWrite) == 0 {
		return true
	}
	return false
}

func (c *Config) checkCanWrite(k string) bool {
	log.Println("api::", k)
	log.Println("Hoooo::", c.RootURL)
	for _, x := range c.APIKeys.ReadWrite {
		if x == k {
			return true
		}
	}
	if len(c.APIKeys.ReadOnly) == 0 && len(c.APIKeys.ReadWrite) == 0 {
		return true
	}
	return false
}
