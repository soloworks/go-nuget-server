package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/url"
	"path/filepath"
)

// Config represents the config file
type Config struct {
	Loglevel  int    `json:"log-level"`
	HostURL   string `json:"host-url"`
	FileStore struct {
		// Type can be 'gcp'|'local'
		Type string `json:"type"`
		// Options for 'local'
		RepoDIR string `json:"local-directory"`
		// Options for 'gcp'
		BucketName string `json:"storage-bucket"`
		ProjectID  string `json:"project-id"`
		// Hard coded API keys
		APIKeys struct {
			ReadOnly  []string `json:"read-only"`
			ReadWrite []string `json:"read-write"`
		} `json:"api-keys"`
	} `json:"filestore"`
}

// Server represents the global server object
type Server struct {
	config           *Config
	URL              *url.URL
	MetaDataResponse []byte
	fs               fileStore
}

// InitServer returns a structure with all core config data
func InitServer(cf string) *Server {
	// Create a new server structure
	s := &Server{}

	// read configuration file
	log.Println(`Loading configuration from "` + cf + `"`)

	data, err := ioutil.ReadFile(cf)
	if err != nil {
		log.Fatal(err)
	}

	// read metadata XML file
	s.MetaDataResponse, err = ioutil.ReadFile(filepath.Join("templates", "$metadata.xml"))
	if err != nil {
		log.Fatal(err)
	}

	// Load in the config file from the file system
	err = json.Unmarshal(data, &s.config)
	if err != nil {
		log.Fatal("Error with json:", err)
	}

	// Set URL
	u, err := url.Parse(s.config.HostURL)
	s.URL = u

	// Init the fileStore
	switch s.config.FileStore.Type {
	case "gcp":
		s.fs = &fileStoreGCP{}
	case "local":
		s.fs = &fileStoreLocal{}
	}
	if err := s.fs.Init(s); err != nil {
		log.Fatal("Error starting FileStore:", err)
	}

	// Todo Warn if API Keys not present
	a, err := s.fs.GetAccessLevel("")
	if err != nil {
		log.Fatal("Error getting AccessLevel", err)
	}
	if a == accessReadWrite {
		log.Println("WARNING: No API Keys defined, server running in development mode")
		log.Println("WARNING: Anyone can read or write to the server")
	} else if a == accessReadOnly {
		log.Println("WARNING: No read-only API Keys defined")
		log.Println("WARNING: Anyone can read from the server")
	}

	return s
}
