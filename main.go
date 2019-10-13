package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var isTLS bool = false
var repo *nugetRepo
var baseURL string

func init() {
	ArgbaseURL := "localhost/plugins/"
	// Set BaseURL
	switch isTLS {
	case true:
		baseURL = `https://` + ArgbaseURL
	case false:
		baseURL = `http://` + ArgbaseURL
	}
}

func main() {

	// Init the file repo
	repo = initRepo(`./Packages`)

	// Handling Routing
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Routing: " + r.URL.String())
		switch {
		case r.URL.String() == `/plugins/`:
			serveRoot(w, r)
		case strings.HasPrefix(r.URL.String(), `/plugins/Packages`):
			serveFeed(w, r)
		case strings.HasPrefix(r.URL.String(), `/plugins/api/v2/package`):
			servePackage(w, r)
		case strings.HasPrefix(r.URL.String(), `/F/plugins/api/v2/browse`):
			// Get file path and split to match local
			f := strings.TrimLeft(r.URL.String(), `/F/plugins/api/v2/browse`)
			http.ServeFile(w, r, filepath.Join(repo.path, `browse`, f))
		}
	})

	// Log and start server
	log.Println("Serving on http://localhost:80")
	log.Fatal(http.ListenAndServe(":80", nil))
}

func serveRoot(w http.ResponseWriter, r *http.Request) {

	// Debug Tracking
	log.Println("Serving Root")

	// Set Headers
	w.Header().Set("Content-Type", "application/xml;charset=utf-8")

	// Create a new Service Struct
	ns := NewNugetService(r.Host + r.RequestURI)

	// Output Xml
	w.Write(ns.ToBytes())
}

func serveFeed(w http.ResponseWriter, r *http.Request) {

	// Debug Tracking
	log.Println("Serving Packages")

	// Set Headers
	w.Header().Set("Content-Type", "application/atom+xml;type=feed;charset=utf-8")

	// Identify & process function parameters if they exist
	log.Println(r.URL.Path)
	var params string
	// Get Params
	i := strings.Index(r.URL.Path, "(")
	if i >= 0 {
		j := strings.Index(r.URL.Path[i:], ")")
		if j >= 0 {
			params = r.URL.Path[i+1 : i+j]
		}
	}
	log.Println(params)

	// Create a new Service Struct
	nf := NewNugetFeed(baseURL)

	// Process all packages
	for _, e := range repo.entry {
		nf.Entry = append(nf.Entry, *e)
	}

	// Output Xml
	b := nf.ToBytes()
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.Write(b)

}

func servePackage(w http.ResponseWriter, r *http.Request) {
	// get the last two parts of the URL
	x := strings.Split(strings.TrimLeft(r.URL.String(), `/plugins/api/v2/package`), `/`)

	// Loop through packages to find the one we need
	for f, p := range repo.entry {
		if p.Properties.ID == x[0] && p.Properties.Version == x[1] {
			// Set header to fix filename on client side
			w.Header().Set("Cache-Control", "max-age=3600")
			w.Header().Set("Content-Disposition", `filename=`+f)
			w.Header().Set("Content-Type", "binary/octet-stream")
			// Serve up the file
			http.ServeFile(w, r, filepath.Join(repo.path, f))
		}
	}
}

func serveContent(w http.ResponseWriter, r *http.Request) {}

func zuluTime(t time.Time) string {
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02dZ", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}
