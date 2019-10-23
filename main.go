package main

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	nuspec "github.com/soloworks/go-nuspec"
)

// Global Variables
var repo *nugetRepo
var c *Config

// Global Constants
const zuluTimeLayout = "2006-01-02T15:04:05Z"

func init() {

	c = NewConfig("nuget-server-config.json", "$metadata.xml")

	// Init the file repo
	r, err := initRepo(c.RepoDIR)
	if err != nil {
		log.Fatal(err)
	}
	repo = r
}

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) Status() int {
	if w.status == 0 {
		return 200
	}
	return w.status
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

func main() {

	// Handling Routing
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		// Create new statusWriter
		sw := statusWriter{ResponseWriter: w}

		log.Println("Request Start:-------------------------------------------------")
		log.Println("    Method:", r.Method)
		log.Println("    Path:", r.URL.String())

		// Local Variables
		var apiKey string
		// Process Headers
		log.Println("    Headers:")
		for name, headers := range r.Header {
			// Grab ApiKey as it passes
			if strings.ToLower(name) == "x-nuget-apikey" {
				apiKey = headers[0]
			}
			for _, h := range headers {
				// Log Key
				log.Println("        " + name + "::" + h)
			}
		}

		// Free Access Routes
		switch r.Method {
		case http.MethodGet:
			switch {
			case r.URL.String() == c.URL.Path:
				serveRoot(&sw, r)
				goto End
			case r.URL.String() == c.URL.Path+`$metadata`:
				serveMetaData(&sw, r)
				goto End
			}
		}

		// Restricted Routes
		switch r.Method {
		case http.MethodGet:
			// Verify API Key allows Reading
			if !c.verifyUserCanReadOnly(apiKey) {
				sw.WriteHeader(http.StatusForbidden)
				goto End
			}
			// Perform Routing
			switch {
			case strings.HasPrefix(r.URL.String(), c.URL.Path+`Refresh`):
				// ToDo: Remove as should be dynamic from mini DB
				repo.RefeshPackages()
			case strings.HasPrefix(r.URL.String(), c.URL.Path+`Packages`):
				servePackageFeed(&sw, r)
			case strings.HasPrefix(r.URL.String(), c.URL.Path+`FindPackagesById`):
				servePackageFeed(&sw, r)
			case strings.HasPrefix(r.URL.String(), c.URL.Path+`nupkg`):
				servePackageFile(&sw, r)
			case strings.HasPrefix(r.URL.String(), c.URL.Path+`files`):
				log.Println("Serve: Static File")
				// Get file path and split to match local
				http.ServeFile(&sw, r, filepath.Join(repo.rootDIR, r.URL.String()[len(c.URL.Path+`files`):]))
			case strings.HasPrefix(r.URL.String(), c.URL.Path+`files`):
				// Catch for client forcing use of "/F/yourpath/api/v2/browse"
				log.Println("Serve: Static File (")

			default:
				sw.WriteHeader(http.StatusNotFound)
				goto End
			}
		case http.MethodPut:
			// Verify API Key allows writing
			if !c.verifyUserCanReadWrite(apiKey) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			// Route
			switch {
			case r.URL.String() == c.URL.Path:
				// Process Request
				uploadPackageFile(&sw, r)
			default:
				// Return 404
				sw.WriteHeader(http.StatusNotFound)
				goto End
			}

		}

	End:
		log.Println("Request Served:", r.Method, r.URL.String())
		log.Println("Status:", sw.Status())
		log.Println("    Headers:")
		if len(w.Header()) == 0 {
			log.Println("        None")
		} else {
			for name, headers := range w.Header() {
				for _, h := range headers {
					// Log Key
					log.Println("        " + name + "::" + h)
				}
			}
		}
		log.Println("Request End:---------------------------------------------------")
	})

	// Log and start server
	log.Println("Server running on ", c.URL.String())
	p := ""
	if c.URL.Port() != "" {
		p = ":" + c.URL.Port()
	}
	log.Fatal(http.ListenAndServe(p, nil))
}

func serveRoot(w http.ResponseWriter, r *http.Request) {

	// Debug Tracking
	log.Println("Serve: Root")

	// Set Headers
	w.Header().Set("Content-Type", "application/xml;charset=utf-8")

	// Create a new Service Struct
	ns := NewNugetService(r.Host + r.RequestURI)

	// Output Xml
	w.Write(ns.ToBytes())
}

func serveMetaData(w http.ResponseWriter, r *http.Request) {

	// Debug Tracking
	log.Println("Serve: MetaData")

	// Set Headers
	w.Header().Set("Content-Type", "application/xml;charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(c.MetaDataResponse)))

	// Output Xml
	w.Write(c.MetaDataResponse)
}

func servePackageFeed(w http.ResponseWriter, r *http.Request) {

	// Identify & process function parameters if they exist
	var s string
	// Get Params
	i := strings.Index(r.URL.Path, "(") // Find opening bracket
	if i >= 0 {
		j := strings.Index(r.URL.Path[i:], ")") // Find closing bracket
		if j >= 0 {
			s = r.URL.Path[i+1 : i+j]
		}
	}
	params := newPackageParams(s)
	var b []byte

	if strings.HasPrefix(r.URL.String(), c.URL.Path+`Packages`) {
		if params.ID != "" && params.Version != "" {
			// Debug Tracking
			log.Println("Serve Package: Entry (" + params.ID + "." + params.Version + ")")
			// Find the entry required
			for _, p := range repo.packages {
				if p.Properties.ID == params.ID && p.Properties.Version == params.Version {
					b = p.ToBytes()
				}
			}
		} else {

			// Process all packages
			s := strings.SplitAfterN(r.URL.Query().Get("$filter"), " ", 3)
			if s[0] == "" {
				log.Println("Serve Package: Feed (Unfiltered)")
			} else {
				log.Println("Serve Package: Feed (Filtered)")
			}

			// Create a new Service Struct
			nf := NewNugetFeed("Packages", c.URL.String())
			// Loop through packages
			for _, p := range repo.packages {
				if s[0] != "" {
					if strings.TrimSpace(s[0]) == "tolower(Id)" && strings.TrimSpace(s[1]) == "eq" {
						if strings.ToLower(p.Properties.ID) == s[2][1:len(s[2])-1] {
							nf.Packages = append(nf.Packages, p)
						}
					}
				} else {
					nf.Packages = append(nf.Packages, p)
				}
			}
			// Output Xml
			b = nf.ToBytes()
		}
	} else if strings.HasPrefix(r.URL.String(), c.URL.Path+`FindPackagesById`) {
		log.Println("Serve Package: Feed (ById)")

		id := r.URL.Query().Get("id")
		id = id[1 : len(id)-1]
		log.Println("id::" + id)

		// Create a new Service Struct
		nf := NewNugetFeed("FindPackagesById", c.URL.String())

		// Loop through packages
		for _, p := range repo.packages {
			log.Println("p.ID =", p.Properties.ID)
			if id == p.Properties.ID {
				nf.Packages = append(nf.Packages, p)
			}
		}
		b = nf.ToBytes()
	}

	if len(b) == 0 {
		w.WriteHeader(404)
	} else {
		// Set Headers
		w.Header().Set("Content-Type", "application/atom+xml;type=feed;charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(b)))
		w.Write(b)
	}

}

func servePackageFile(w http.ResponseWriter, r *http.Request) {
	// get the last two parts of the URL
	x := strings.Split(r.URL.String(), `/`)
	// construct filename of desired package
	filename := x[len(x)-2] + "." + x[len(x)-1] + ".nupkg"
	// Debug Tracking
	log.Println("Serving Package", filename)

	// Loop through packages to find the one we need
	for _, p := range repo.packages {
		if p.Properties.ID == x[len(x)-2] && p.Properties.Version == x[len(x)-1] {
			// Set header to fix filename on client side
			w.Header().Set("Cache-Control", "max-age=3600")
			w.Header().Set("Content-Disposition", `filename=`+p.filename)
			w.Header().Set("Content-Type", "binary/octet-stream")
			// Serve up the file
			http.ServeFile(w, r, filepath.Join(repo.rootDIR, p.Properties.ID, p.Properties.Version, p.filename))
		}
	}
}

func uploadPackageFile(w http.ResponseWriter, r *http.Request) {

	log.Println("Puitting Package into Store")

	// Parse mime type
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		log.Fatal(err)
	}

	// Setup byte array to hold zip body
	var body []byte

	// Detect and Decode based on mime type
	if strings.HasPrefix(mediaType, "multipart/form-data") {
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Println(err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			body, err = ioutil.ReadAll(p)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}

	// Bail if length is zero
	if len(body) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Try and open the attachment as a zip file
	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		println(err.Error())
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	var nsf *nuspec.File

	// Find and Process the .nuspec file
	for _, zipFile := range zipReader.File {
		// If this is the root .nuspec file read it into a NewspecFile structure
		if filepath.Dir(zipFile.Name) == "." && filepath.Ext(zipFile.Name) == ".nuspec" {
			// Marshall XML into Structure
			rc, err := zipFile.Open()
			if err != nil {
				println(err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// Read into NuspecFile structure
			nsf, err = nuspec.FromReader(rc)
		}
	}

	// Test for folder, if present bail, if not make it
	packagePath := filepath.Join(c.RepoDIR, strings.ToLower(nsf.Meta.ID), nsf.Meta.Version)
	if _, err := os.Stat(packagePath); !os.IsNotExist(err) {
		// Path already exists
		w.WriteHeader(http.StatusConflict)
		return
	}
	err = os.MkdirAll(packagePath, os.ModePerm)
	if err != nil {
		println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Println("Creating Directory: ", packagePath)

	// Extract contents to folder
	for _, zipFile := range zipReader.File {
		if _, err := os.Stat(zipFile.Name); os.IsNotExist(err) {
			// Create directory for file if not present
			fd := filepath.Join(packagePath, filepath.Dir(zipFile.Name))
			if _, err := os.Stat(fd); os.IsNotExist(err) {
				err = os.MkdirAll(fd, os.ModePerm)
				if err != nil {
					println(err.Error())
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			// Set the file path
			fp := filepath.Join(fd, filepath.Base((zipFile.Name)))
			if _, err := os.Stat(fp); os.IsNotExist(err) {

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

	// Dump the .nupkg file in the same directory
	err = ioutil.WriteFile(filepath.Join(packagePath, strings.ToLower(nsf.Meta.ID)+"."+nsf.Meta.Version+".nupkg"), body, os.ModePerm)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Println("Package Saved: ", packagePath)
}
