package main

import "net/url"

type config struct {
	HostURL string `json:"host-url"`
	RepoDIR string `json:"package-directory"`
	APIKeys struct {
		ReadOnly  []string `json:"read-only"`
		ReadWrite []string `json:"read-write"`
	} `json:"api-keys"`
	URL *url.URL
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
