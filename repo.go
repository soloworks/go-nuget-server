package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"encoding/xml"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

type nugetRepo struct {
	path  string
	entry map[string]*NugetFeedEntry
}

func initRepo(repoPath string) *nugetRepo {
	// Create a new repo structure
	r := nugetRepo{}
	// Init the package map
	r.entry = make(map[string]*NugetFeedEntry)
	// Set the Repo Path
	r.path = repoPath
	// Read in all files in directory
	files, err := ioutil.ReadDir(r.path)
	if err != nil {
		log.Fatal(err)
	}

	// Loop through all files
	for _, f := range files {
		// Check if file is a NuPkg
		if filepath.Ext(f.Name()) == ".nupkg" {
			r.AddPackage(f)
		}
	}

	log.Printf("%d Packages Found", len(r.entry))

	// Return repo
	return &r
}

func (r *nugetRepo) AddPackage(f os.FileInfo) {

	// Open and read in the file (Is a Zip file under the hood)
	content, err := ioutil.ReadFile(filepath.Join(r.path, f.Name()))
	if err != nil {
		log.Fatal(err)
	}
	// Set up a zipReader
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		log.Fatal(err)
	}

	// Find and Process the .nuspec file
	for _, zipFile := range zipReader.File {
		// If this is the root .nuspec file read it into a NewspecFile structure
		if filepath.Dir(zipFile.Name) == "." && filepath.Ext(zipFile.Name) == ".nuspec" {
			// Marshall XML into Structure
			rc, err := zipFile.Open()
			if err != nil {
				log.Fatal(err)
			}
			b, err := ioutil.ReadAll(rc)
			if err != nil {
				log.Fatal(err)
			}
			// Read into NuspecFile structure
			var nsf NuspecFile
			err = xml.Unmarshal(b, &nsf)

			// Read Entry into memory
			r.entry[f.Name()] = NewNugetFeedEntry(baseURL, nsf)
			// Set Updated to match file
			r.entry[f.Name()].Properties.Created.Value = zuluTime(f.ModTime())
			r.entry[f.Name()].Properties.LastEdited.Value = zuluTime(f.ModTime())
			r.entry[f.Name()].Properties.Published.Value = zuluTime(f.ModTime())
			r.entry[f.Name()].Updated = zuluTime(f.ModTime())
			// Get and Set file hash
			h := sha512.Sum512(content)
			r.entry[f.Name()].Properties.PackageHash = hex.EncodeToString(h[:])
			r.entry[f.Name()].Properties.PackageHashAlgorithm = `SHA512`
			r.entry[f.Name()].Properties.PackageSize.Value = len(content)
			r.entry[f.Name()].Properties.PackageSize.Type = "Edm.Int64"
		}
	}

	// Create a content folder entry if not already present
	cd := filepath.Join(r.path, `browse`, r.entry[f.Name()].Properties.ID, r.entry[f.Name()].Properties.Version)
	if _, err := os.Stat(cd); os.IsNotExist(err) {
		log.Println("Creating: " + cd)
		os.MkdirAll(cd, os.ModePerm)
	}

	// Process the content files
	for _, zipFile := range zipReader.File {
		if _, err := os.Stat(zipFile.Name); os.IsNotExist(err) {
			// Create directory for file if not present
			fd := filepath.Join(cd, filepath.Dir(zipFile.Name))
			if _, err := os.Stat(fd); os.IsNotExist(err) {
				log.Println("Creating: " + fd)
				os.MkdirAll(fd, os.ModePerm)
			}

			// Set the file path
			fp := filepath.Join(fd, filepath.Base((zipFile.Name)))
			if _, err := os.Stat(fp); os.IsNotExist(err) {

				// Log Out Status
				log.Println("Extracting: " + fp)

				// Open file to be extracted
				r, err := zipFile.Open()
				if err != nil {
					log.Fatal(err)
				}

				// Create the file
				outFile, err := os.Create(fp)
				if err != nil {
					log.Fatal(err)
				}
				defer outFile.Close()
				// Dump bytes into file
				_, err = io.Copy(outFile, r)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}

func (r *nugetRepo) RemovePackage(f os.FileInfo) {
	// Remove the Package from the Map
	delete(r.entry, f.Name())
	// Delete the contents directory
	os.RemoveAll(filepath.Join(r.path, `content`, f.Name()))
}
