package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/url"
)

// Config represents the server configuration and other data
type Config struct {
	HostURL string `json:"host-url"`
	RepoDIR string `json:"package-directory"`
	APIKeys struct {
		ReadOnly  []string `json:"read-only"`
		ReadWrite []string `json:"read-write"`
	} `json:"api-keys"`
	URL              *url.URL
	MetaDataResponse []byte
}

// NewConfig returns a structure with all core config data
func NewConfig(cf string, mdf string) *Config {
	// Create a new server structure
	c = &Config{}

	// read configuration file
	log.Println(`Loading configuration from "nuget-server-config.json"`)

	data, err := ioutil.ReadFile(cf)
	if err != nil {
		log.Fatal(err)
	}

	// read metadata XML file
	c.MetaDataResponse, err = ioutil.ReadFile(mdf)
	if err != nil {
		log.Fatal(err)
	}

	// Load in the config file from the file system
	err = json.Unmarshal(data, &c)
	if err != nil {
		log.Fatal("Error with json:", err)
	}

	// TODO: Remove any empty APIKeys

	// Set URL
	u, err := url.Parse(c.HostURL)
	c.URL = u

	// Warn if API Keys not present
	if len(c.APIKeys.ReadOnly) == 0 && len(c.APIKeys.ReadWrite) == 0 {
		log.Println("WARNING: No API Keys defined, server running in development mode")
		log.Println("WARNING: Anyone can read or write to the server")
	} else if len(c.APIKeys.ReadOnly) == 0 {
		log.Println("WARNING: No read-only API Keys defined")
		log.Println("WARNING: Anyone can read from the server")
	}

	return c
}

func (c *Config) verifyUserCanReadWrite(k string) bool {
	// Shortcut if no api-keys are present
	for _, x := range c.APIKeys.ReadWrite {
		if x == k {
			return true
		}
	}
	return false
}

func (c *Config) verifyUserCanReadOnly(k string) bool {
	if len(c.APIKeys.ReadOnly) == 0 {
		return true
	}
	for _, x := range c.APIKeys.ReadOnly {
		if x == k {
			return true
		}
	}
	if c.verifyUserCanReadWrite(k) {
		return true
	}
	return false
}
