package main

import (
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

// Global Variables
var server *Server

func init() {
	// Loan config and init server
	server = InitServer("nuget-server-config-gcp.json")
}

func main() {

	// Handling Routing
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		// Local Varibles
		var err error               // Reusable error
		apiKey := ""                // APIKey (populated if found in headers)
		accessLevel := accessDenied // Access Level (defaults to denied)

		// Create new statusWriter
		sw := statusWriter{ResponseWriter: w}

		// Open Access Routes (No ApiKey needed)
		switch r.Method {
		case http.MethodGet:
			switch {
			case r.URL.String() == server.URL.Path:
				serveRoot(&sw, r)
				goto End
			case r.URL.String() == server.URL.Path+`$metadata`:
				serveMetaData(&sw, r)
				goto End
			}
		}

		// Process Headers looking for API key (can't access direct as case may not match)
		for name, headers := range r.Header {
			// Grab ApiKey as it passes
			if strings.ToLower(name) == "x-nuget-apikey" {
				apiKey = headers[0]
			}
		}
		accessLevel, err = server.fs.GetAccessLevel(apiKey)
		if err != nil {
			sw.WriteHeader(http.StatusInternalServerError)
			goto End
		}
		// Bounce any unauthorised requests
		if accessLevel == accessDenied {
			sw.WriteHeader(http.StatusForbidden)
			goto End
		}

		// Restricted Routes
		switch r.Method {
		case http.MethodGet:
			// Perform Routing
			altFilePath := path.Join(`/F`, server.URL.Path, `api`, `v2`, `browse`)
			switch {
			case strings.HasPrefix(r.URL.String(), server.URL.Path+`Packages`):
				servePackageFeed(&sw, r)
			case strings.HasPrefix(r.URL.String(), server.URL.Path+`FindPackagesById`):
				servePackageFeed(&sw, r)
			case strings.HasPrefix(r.URL.String(), server.URL.Path+`nupkg`):
				servePackageFile(&sw, r)
			case strings.HasPrefix(r.URL.String(), server.URL.Path+`files`):
				serveStaticFile(&sw, r, r.URL.String()[len(server.URL.Path+`files`):])
			case strings.HasPrefix(r.URL.String(), altFilePath):
				serveStaticFile(&sw, r, r.URL.String()[len(altFilePath):])
			default:
				sw.WriteHeader(http.StatusNotFound)
				goto End
			}
		case http.MethodPut:
			// Bounce any request without write accees
			if accessLevel != accessReadWrite {
				sw.WriteHeader(http.StatusForbidden)
				return
			}

			// Route
			switch {
			case r.URL.String() == server.URL.Path:
				// Process Request
				uploadPackage(&sw, r)
			default:
				sw.WriteHeader(http.StatusNotFound)
				goto End
			}
		default:
			sw.WriteHeader(http.StatusNotFound)
			goto End
		}

	End:

		log.Println("Request::", sw.Status(), r.Method, r.URL.String())

		if server.config.Loglevel > 0 {
			log.Println("Request Headers:")
			if len(w.Header()) == 0 {
				log.Println("        None")
			} else {
				for name, headers := range r.Header {
					for _, h := range headers {
						// Log Key
						log.Println("        " + name + "::" + h)
					}
				}
			}

			log.Println("Response Headers:")
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
		}
	})

	// Set port number (Defaults to 80)
	p := ""
	// if port is set in URL string
	if server.URL.Port() != "" {
		p = ":" + server.URL.Port()
	}
	// If PORT EnvVar is set (Google Cloud Run environment)
	if os.Getenv("PORT") != "" {
		p = ":" + os.Getenv("PORT")
	}

	// Log and Start server
	log.Println("Starting Server on ", server.URL.String()+p)
	log.Fatal(http.ListenAndServe(p, nil))
}

func serveRoot(w http.ResponseWriter, r *http.Request) {

	// Create a new Service Struct
	ns := NewNugetService(server.URL.String())
	b := ns.ToBytes()

	// Set Headers
	w.Header().Set("Content-Type", "application/xml;charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))

	// Output Xml
	w.Write(b)
}

func serveMetaData(w http.ResponseWriter, r *http.Request) {

	// Set Headers
	w.Header().Set("Content-Type", "application/xml;charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(server.MetaDataResponse)))

	// Output Xml
	w.Write(server.MetaDataResponse)
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, fn string) {

	// Get the file from the FileStore
	b, err := server.fs.GetFile(fn)
	if err == ErrFileNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Set Headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))

	// Output Xml
	w.Write(b)
}

func servePackageFile(w http.ResponseWriter, r *http.Request) {

	// get the last two parts of the URL
	x := strings.Split(r.URL.String(), `/`)
	// construct filename of desired package
	fn := x[len(x)-2] + "." + x[len(x)-1] + ".nupkg"
	p := path.Join(x[len(x)-2], x[len(x)-1], fn)
	b, err := server.fs.GetFile(p)
	if err == ErrFileNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return

	}

	// Set header to fix filename on client side
	w.Header().Set("Cache-Control", "max-age=3600")
	w.Header().Set("Content-Disposition", `filename=`+fn)
	w.Header().Set("Content-Type", "binary/octet-stream")
	// Serve up the file
	w.Write(b)
}

func servePackageFeed(w http.ResponseWriter, r *http.Request) {

	// Local Variables
	var err error
	var b []byte
	var params = &packageParams{}

	// Identify & process function parameters if they exist
	if i := strings.Index(r.URL.Path, "("); i >= 0 { // Find opening bracket
		if j := strings.Index(r.URL.Path[i:], ")"); j >= 0 { // Find closing bracket
			params = newPackageParams(r.URL.Path[i+1 : i+j])
		}
	}

	// For /Packages() Route
	if strings.HasPrefix(r.URL.String(), server.URL.Path+`Packages`) {
		// If params are populated then this is a single entry requests
		if params.ID != "" && params.Version != "" {
			// Find the entry required
			npe, err := server.fs.GetPackage(params.ID, params.Version)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Convert it to Bytes
			b = npe.ToBytes()
		} else {
			// Create a new Service Struct
			nf := NewNugetFeed("Packages", server.URL.String())

			// Split out weird filter formatting
			s := strings.SplitAfterN(r.URL.Query().Get("$filter"), " ", 3)

			// Create empty id string
			id := ""

			// If relevant, repopulate id with
			if strings.TrimSpace(s[0]) == "tolower(Id)" && strings.TrimSpace(s[1]) == "eq" {
				id = s[2]
			}

			// Populate Packages from FileStore
			nf.Packages, err = server.fs.GetPackages(id)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Output Xml
			b = nf.ToBytes()
		}
	} else if strings.HasPrefix(r.URL.String(), server.URL.Path+`FindPackagesById`) {

		// Get ID from query
		id := r.URL.Query().Get("id") // Get Value
		id = id[1 : len(id)-1]        // Remove Quotes

		// Create a new Service Struct
		nf := NewNugetFeed("FindPackagesById", server.URL.String())

		// Populate Packages from FileStore
		nf.Packages, err = server.fs.GetPackages(id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Output Xml
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

func uploadPackage(w http.ResponseWriter, r *http.Request) {

	log.Println("Putting Package into FileStore")

	// Parse Mime type
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Detect and Decode based on mime type
	if strings.HasPrefix(mediaType, "multipart/form-data") {
		// Get a multipart.Reader
		mr := multipart.NewReader(r.Body, params["boundary"])
		// Itterate over parts/files uploaded
		for {
			// Get he next part from the multipart.Reader, exit loop if no more
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// Store the package file in byte array for use
			pkgFile, err := ioutil.ReadAll(p)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// Store the file
			exists, err := server.fs.StorePackage(pkgFile)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if exists == true {
				w.WriteHeader(http.StatusConflict)
				return
			}
			w.WriteHeader(http.StatusCreated)
		}
	}
}
