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

	// Todo Warn if API Keys not present
	if len(s.config.FileStore.APIKeys.ReadOnly) == 0 && len(s.config.FileStore.APIKeys.ReadWrite) == 0 {
		log.Println("WARNING: No API Keys defined, server running in development mode")
		log.Println("WARNING: Anyone can read or write to the server")
	} else if len(s.config.FileStore.APIKeys.ReadOnly) == 0 {
		log.Println("WARNING: No read-only API Keys defined")
		log.Println("WARNING: Anyone can read from the server")
	}

	// Init the fileStore
	switch s.config.FileStore.Type {
	case "gcp":
		s.fs = &fileStoreGCP{}
	case "local":
		s.fs = &fileStoreLocal{}
	}
	s.fs.Init(s)

	return s
}

func (s *Server) verifyUserCanReadWrite(k string) bool {
	// Shortcut if no api-keys are present
	for _, x := range s.config.FileStore.APIKeys.ReadWrite {
		if x == k {
			return true
		}
	}
	return false
}

func (s *Server) verifyUserCanReadOnly(k string) bool {
	if len(s.config.FileStore.APIKeys.ReadOnly) == 0 {
		return true
	}
	for _, x := range s.config.FileStore.APIKeys.ReadOnly {
		if x == k {
			return true
		}
	}
	if s.verifyUserCanReadWrite(k) {
		return true
	}
	return false
}
