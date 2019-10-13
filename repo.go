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
	"sort"
)

type nugetRepo struct {
	packagePath string
	packages    []*NugetPackage
}

func initRepo(repoPath string) *nugetRepo {
	// Create a new repo structure
	r := nugetRepo{}
	// Set the Repo Path
	r.packagePath = repoPath

	// Refresh Packages
	r.RefeshPackages()

	// Return repo
	return &r
}

func (r *nugetRepo) AddPackage(f os.FileInfo) {

	// Check if this file is already stored
	for _, p := range r.packages {
		if p.Filename == filepath.Base(f.Name()) {
			return
		}
	}

	// Open and read in the file (Is a Zip file under the hood)
	content, err := ioutil.ReadFile(filepath.Join(r.packagePath, f.Name()))
	if err != nil {
		log.Fatal(err)
	}
	// Set up a zipReader
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		log.Fatal(err)
	}

	// NugetPackage Object
	var p *NugetPackage

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
			p = NewNugetPackage(baseURL, nsf, f.Name())

			// Set Updated to match file
			p.Properties.Created.Value = zuluTime(f.ModTime())
			p.Properties.LastEdited.Value = zuluTime(f.ModTime())
			p.Properties.Published.Value = zuluTime(f.ModTime())
			p.Updated = zuluTime(f.ModTime())
			// Get and Set file hash
			h := sha512.Sum512(content)
			p.Properties.PackageHash = hex.EncodeToString(h[:])
			p.Properties.PackageHashAlgorithm = `SHA512`
			p.Properties.PackageSize.Value = len(content)
			p.Properties.PackageSize.Type = "Edm.Int64"
			// Insert this into the array in order
			index := sort.Search(len(r.packages), func(i int) bool { return r.packages[i].Filename > p.Filename })
			x := NugetPackage{}
			r.packages = append(r.packages, &x)
			copy(r.packages[index+1:], r.packages[index:])
			r.packages[index] = p
		}
	}

	// Create a content folder entry if not already present
	cd := filepath.Join(r.packagePath, `browse`, p.Properties.ID, p.Properties.Version)
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

func (r *nugetRepo) RemovePackage(fn string) {
	// Remove the Package from the Map
	for i, p := range r.packages {
		if p.Filename == fn {
			r.packages = append(r.packages[:i], r.packages[i+1:]...)
		}
	}
	// Delete the contents directory
	os.RemoveAll(filepath.Join(r.packagePath, `content`, fn))
}

func (r *nugetRepo) RefeshPackages() {

	// Read in all files in directory
	files, err := ioutil.ReadDir(r.packagePath)
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

	log.Printf("%d Packages Found", len(r.packages))
}
