package main

import (
	"fmt"
	"log"
	"net/http"
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
			pathRoot(w, r)

		case strings.HasPrefix(r.URL.String(), `/plugins/Packages`):
			pathPackages(w, r)

		case strings.HasPrefix(r.URL.String(), `/plugins/xPackages`):
			log.Println("Serving static file")
			w.Header().Set("Content-Type", "application/atom+xml;type=feed;charset=utf-8")
			http.ServeFile(w, r, "./Samples/Q-Sys-Nuget-PackagesSingle2.xml")
			//http.ServeFile(w, r, "./Samples/Q-Sys-Nuget-Packages.xml")
		}
	})

	// Log and start server
	log.Println("Serving on http://localhost:80")
	log.Fatal(http.ListenAndServe(":80", nil))
}

func pathRoot(w http.ResponseWriter, r *http.Request) {

	// Debug Tracking
	log.Println("Serving Root")

	// Set Headers
	w.Header().Set("Content-Type", "application/xml;charset=utf-8")

	// Create a new Service Struct
	ns := NewNugetService(r.Host + r.RequestURI)

	// Output Xml
	w.Write(ns.ToBytes())
}

func pathPackages(w http.ResponseWriter, r *http.Request) {

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
	println("baseURL::" + baseURL)
	println("parms::" + params)

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

func zuluTime(t time.Time) string {
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02dZ", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}
