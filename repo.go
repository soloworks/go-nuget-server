package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"

	nuspec "github.com/soloworks/go-nuspec"
)

type nugetRepo struct {
	rootDIR  string
	packages []*NugetPackage
}

func initRepo(rootDIR string) (*nugetRepo, error) {
	// Create a new repo structure
	r := nugetRepo{}

	// Set the Repo Path
	r.rootDIR = rootDIR

	// Create the package folder if requried
	if _, err := os.Stat(r.rootDIR); os.IsNotExist(err) {
		// Path already exists
		log.Println("Creating Directory: ", r.rootDIR)
		err := os.MkdirAll(r.rootDIR, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}

	// Refr esh Packages
	err := r.RefeshPackages()
	if err != nil {
		return nil, err
	}

	// Return repo
	return &r, nil
}

func (r *nugetRepo) RefeshPackages() error {

	// Read in all files in directory root
	IDs, err := ioutil.ReadDir(r.rootDIR)
	if err != nil {
		return err
	}

	// Loop through all directories (first level is lowercase IDs)
	for _, ID := range IDs {
		// Check if this is a directory
		if ID.IsDir() {
			// Search files in directory (second level is versions)
			Vers, err := ioutil.ReadDir(filepath.Join(r.rootDIR, ID.Name()))
			if err != nil {
				return err
			}
			for _, Ver := range Vers {
				// Check if this is a directory
				if Ver.IsDir() {
					// Create full filepath
					fp := filepath.Join(r.rootDIR, ID.Name(), Ver.Name(), ID.Name()+"."+Ver.Name()+".nupkg")
					log.Println("Reading: ", fp)
					if _, err := os.Stat(fp); os.IsNotExist(err) {
						log.Println("Not a nupkg directory")
						break
					}
					err = r.LoadPackage(fp)
					if err != nil {
						log.Println("Error: Cannot load package")
						log.Println(err)
						break
					}
					println("Read: ", fp)
				}
			}
		}
	}

	log.Printf("%d Packages Found", len(r.packages))

	return nil
}

func (r *nugetRepo) LoadPackage(fp string) error {

	// Open and read in the file (Is a Zip file under the hood)
	content, err := ioutil.ReadFile(fp)
	if err != nil {
		return err
	}

	f, err := os.Stat(fp)
	if err != nil {
		return err
	}

	// Set up a zipReader
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return err
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
				return err
			}
			b, err := ioutil.ReadAll(rc)
			if err != nil {
				return err
			}
			// Read into NuspecFile structure
			nsf, err := nuspec.FromBytes(b)

			// Read Entry into memory
			p = NewNugetPackage(c.HostURL, nsf, f.Name())

			// Set Updated to match file
			p.Properties.Created.Value = f.ModTime().Format(zuluTimeLayout)
			p.Properties.LastEdited.Value = f.ModTime().Format(zuluTimeLayout)
			p.Properties.Published.Value = f.ModTime().Format(zuluTimeLayout)
			p.Updated = f.ModTime().Format(zuluTimeLayout)
			// Get and Set file hash
			h := sha512.Sum512(content)
			p.Properties.PackageHash = hex.EncodeToString(h[:])
			p.Properties.PackageHashAlgorithm = `SHA512`
			p.Properties.PackageSize.Value = len(content)
			p.Properties.PackageSize.Type = "Edm.Int64"
			// Insert this into the array in order
			index := sort.Search(len(r.packages), func(i int) bool { return r.packages[i].filename > p.filename })
			x := NugetPackage{}
			r.packages = append(r.packages, &x)
			copy(r.packages[index+1:], r.packages[index:])
			r.packages[index] = p
		}
	}

	return nil
}

func (r *nugetRepo) RemovePackage(fn string) {
	// Remove the Package from the Map
	for i, p := range r.packages {
		if p.filename == fn {
			r.packages = append(r.packages[:i], r.packages[i+1:]...)
		}
	}
	// Delete the contents directory
	os.RemoveAll(filepath.Join(r.rootDIR, `content`, fn))
}
