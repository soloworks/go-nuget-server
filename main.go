package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var repo *nugetRepo
var c *Config

func init() {

	// Create a new server structure
	c = &Config{
		RootDIR: ".", // Default to current working directory
		RootURL: "/", // Default to host root
	}

	// Set the server file system root from Environemnt Variable: NUGET_SERVER_ROOT
	if r := os.Getenv("NUGET_SERVER_ROOT"); r != "" {
		c.RootDIR = r
	}
	// read file
	cf := filepath.Join(c.RootDIR, "nuget-server-config.json")
	log.Println("loading config: " + cf)
	data, err := ioutil.ReadFile(cf)
	if err != nil {
		log.Fatal(err)
	}
	// Load in the config file from the file system
	err = json.Unmarshal(data, &c)
	if err != nil {
		log.Fatal("Error with json:", err)
	}

	// Warn if API Keys not present
	if len(c.APIKeys.ReadOnly) == 0 && len(c.APIKeys.ReadWrite) == 0 {
		log.Println("WARNING: No API Keys defined, server running in free access mode")
	}
}

func main() {

	// Init the file repo
	repo = initRepo(`./Packages`)

	// Handling Routing
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Local Variables
		var apiKey string
		// Process Headers
		for name, headers := range r.Header {
			// Find and store APIKey for this request
			if strings.ToLower(name) == "x-nuget-apikey" {
				apiKey = headers[0]
			}
		}
		println(apiKey)
		switch r.Method {
		case http.MethodGet:
			log.Println("Routing GET: " + r.URL.String())
			switch {
			case r.URL.String() == `/plugins/`:
				serveRoot(w, r)
			case strings.HasPrefix(r.URL.String(), `/plugins/Refresh`):
				repo.RefeshPackages()
			case strings.HasPrefix(r.URL.String(), `/plugins/Packages`):
				serveFeed(w, r)
			case strings.HasPrefix(r.URL.String(), `/plugins/api/v2/package`):
				servePackage(w, r)
			case strings.HasPrefix(r.URL.String(), `/F/plugins/api/v2/browse`):
				log.Println("Serving File")
				// Get file path and split to match local
				f := strings.TrimLeft(r.URL.String(), `/F/plugins/api/v2/browse`)
				http.ServeFile(w, r, filepath.Join(repo.packagePath, `browse`, f))
			}
		case http.MethodPost:
			log.Println("Routing POST: " + r.URL.String())
		case http.MethodPut:
			log.Println("Routing PUT: " + r.URL.String())
			if !c.checkCanWrite(apiKey) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
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

type packageParams struct {
	ID      string
	Version string
}

func newPackageParams(p string) *packageParams {
	pp := packageParams{}

	for strings.Contains(p, `=`) {
		i := strings.Index(p, `=`)
		k := strings.TrimSpace(p[:i])
		p = p[i:]
		i = strings.Index(p, `'`)
		j := strings.Index(p[i+1:], `'`)
		v := strings.TrimSpace(p[i+1 : j+i+1])
		p = strings.TrimSpace(p[j+i+2:])
		if strings.HasPrefix(p, ",") {
			p = p[1:]
		}
		switch k {
		case `Id`:
			pp.ID = v
		case `Version`:
			pp.Version = v
		}
		//output = append(output[:i], append([]byte(` /`), output[i+j+1:]...)...)
	}

	return &pp
}

func serveFeed(w http.ResponseWriter, r *http.Request) {

	// Debug Tracking
	log.Println("Serving Feed")

	// Set Headers
	w.Header().Set("Content-Type", "application/atom+xml;type=feed;charset=utf-8")

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

	if params.ID != "" && params.Version != "" {
		log.Println("Serving Single Entry")
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
			log.Println("Serving Full Feed")
		} else {
			log.Println("Serving Filtered Feed")
		}

		// Create a new Service Struct
		nf := NewNugetFeed(c.baseURL(r))
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

	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.Write(b)

}

func servePackage(w http.ResponseWriter, r *http.Request) {
	// Debug Tracking
	log.Println("Serving Package")
	// get the last two parts of the URL
	x := strings.Split(strings.TrimLeft(r.URL.String(), `/plugins/api/v2/package`), `/`)

	// Loop through packages to find the one we need
	for _, p := range repo.packages {
		if p.Properties.ID == x[0] && p.Properties.Version == x[1] {
			// Set header to fix filename on client side
			w.Header().Set("Cache-Control", "max-age=3600")
			w.Header().Set("Content-Disposition", `filename=`+p.Filename)
			w.Header().Set("Content-Type", "binary/octet-stream")
			// Serve up the file
			http.ServeFile(w, r, filepath.Join(repo.packagePath, p.Filename))
		}
	}
}

func serveContent(w http.ResponseWriter, r *http.Request) {}

var zuluTimeLayout = "2006-01-02T15:04:05Z"

func zuluTime(t time.Time) string {
	//return t.Format(zuluTimeLayout)
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02dZ", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}
