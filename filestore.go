package main

import (
	"archive/zip"
	"bytes"
	"io/ioutil"
	"path/filepath"

	nuspec "github.com/soloworks/go-nuspec"
)

// Global Constant for formatting time strings
const zuluTimeLayout = "2006-01-02T15:04:05Z"

type fileStore interface {
	Init(c *Server) error
	GetPackageEntry(id string, ver string) (*NugetPackageEntry, error)
	GetPackageFeedEntries(id string, startAfter string, max int) ([]*NugetPackageEntry, error)
	StorePackage(pkg []byte) (bool, error)
	GetFile(f string) ([]byte, string, error)
	GetPackageFile(id string, ver string) ([]byte, string, error)
	GetAccessLevel(key string) (access, error)
}

func extractPackage(pkg []byte) (*nuspec.File, map[string][]byte, error) {

	// Open package data as zipfile
	zipReader, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		return nil, nil, err
	}

	// values to be returned
	var nsf *nuspec.File
	files := make(map[string][]byte)

	// Find and Process the .nuspec file within the zip
	for _, zippedFile := range zipReader.File {
		// If this is the root .nuspec file read it into a NewspecFile structure
		if filepath.Dir(zippedFile.Name) == "." && filepath.Ext(zippedFile.Name) == ".nuspec" {
			// Get a reader for this file
			rc, err := zippedFile.Open()
			if err != nil {
				return nil, nil, err
			}
			// Read into nuspec.File structure
			nsf, err = nuspec.FromReader(rc)
		}
	}

	// Extract contents to files
	for _, zipFile := range zipReader.File {

		// Open file to be extracted
		r, err := zipFile.Open()
		if err != nil {
			return nil, nil, err
		}
		// Read all bytes
		b, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, nil, err
		}
		// Store in map with filename
		files[zipFile.Name] = b
	}

	// return elements
	return nsf, files, nil
}

// FileStoreError represents a FileStore Error
type FileStoreError struct {
	ErrorString string
}

func (fse *FileStoreError) Error() string {
	return fse.ErrorString
}

var (
	// ErrFileNotFound is returned when request file is not found in the store
	ErrFileNotFound = &FileStoreError{"File Not Found"}
)

// Access Types for ease of reference
type access int

const (
	// AccessDenied returned when no access to resource is granted
	accessDenied access = iota
	// AccessReadOnly returned when Read access to resouce is granted
	accessReadOnly
	// AccessReadWrite returned when Read and Write to resouce is granted
	accessReadWrite
)
