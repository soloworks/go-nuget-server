package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	nuspec "github.com/soloworks/go-nuspec"
)

// Global Variables
var repo *nugetRepo
var cfg *config

// Global Constants
const zuluTimeLayout = "2006-01-02T15:04:05Z"

func init() {

	// Create a new server structure
	cfg = &config{}

	// read file
	log.Println("loading config: " + "nuget-server-config.json")
	data, err := ioutil.ReadFile("nuget-server-config.json")
	if err != nil {
		log.Fatal(err)
	}

	// Load in the config file from the file system
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		log.Fatal("Error with json:", err)
	}

	// TODO: Remove any empty APIKeys

	// Set URL
	u, err := url.Parse(cfg.HostURL)
	cfg.URL = u

	// Warn if API Keys not present
	if len(cfg.APIKeys.ReadOnly) == 0 && len(cfg.APIKeys.ReadWrite) == 0 {
		log.Println("WARNING: No API Keys defined, server running in development mode")
		log.Println("WARNING: Anyone can read or write to the server")
	} else if len(cfg.APIKeys.ReadOnly) == 0 {
		log.Println("WARNING: No read-only API Keys defined")
		log.Println("WARNING: Anyone can read from the server")
	}

	// Init the file repo
	repo, err = initRepo(cfg.RepoDIR)
	if err != nil {
		log.Fatal(err)
	}
}

func canRead() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("Checking Read")
		// Local Variables
		apiKey := c.GetHeader("x-nuget-apikey")
		log.Println("apiKey:", apiKey)
		// Process Headers
		if cfg.verifyUserCanRead(apiKey) {
			c.Next()
		} else {
			c.AbortWithStatus(403)
		}
	}
}

func canWrite() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("Checking Write")
		// Local Variables
		apiKey := c.GetHeader("x-nuget-apikey")
		// Process Headers
		if cfg.verifyUserCanWrite(apiKey) {
			c.Next()
		}
	}
}

func main() {

	// Create new gin router
	router := gin.Default()

	// Static Routing
	router.Static("/files", repo.rootDIR)
	router.NoRoute()
	// GET Routing
	log.Println("Path::", cfg.URL.Path)
	readOnly := router.Group(cfg.URL.Path)
	readOnly.Use(canRead())
	{
		readOnly.GET("/", serveRoot)
		readOnly.GET("/nupkg/", servePackage)
		readOnly.GET("*Package", serveFeed)
		// readOnly.NoRooute // To handle Packages?
	}

	// PUT Routing
	readWrite := router.Group(cfg.URL.Path)
	readWrite.Use(canWrite())
	{
		readWrite.PUT("/", putPackage)
	}

	router.Run(":80")
}

func serveRoot(c *gin.Context) {

	// Debug Tracking
	log.Println("Serving Root")

	// Create a new Service Struct
	ns := NewNugetService(cfg.HostURL)

	// Output Xml
	c.Data(200, "application/xml;charset=utf-8", ns.ToBytes())
}

func serveFeed(c *gin.Context) {

	// Debug Tracking
	log.Println("Serving Feed")

	// Identify & process function parameters if they exist
	var s string
	// Get Params
	i := strings.Index(c.Request.URL.Path, "(") // Find opening bracket
	if i >= 0 {
		j := strings.Index(c.Request.URL.Path[i:], ")") // Find closing bracket
		if j >= 0 {
			s = c.Request.URL.Path[i+1 : i+j]
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
		s := strings.SplitAfterN(c.Request.URL.Query().Get("$filter"), " ", 3)
		if s[0] == "" {
			log.Println("Serving Full Feed")
		} else {
			log.Println("Serving Filtered Feed")
		}

		// Create a new Service Struct
		nf := NewNugetFeed(cfg.URL.String())
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

	// Set Headers
	c.Data(200, "application/atom+xml;type=feed;charset=utf-8", b)

}

func servePackage(c *gin.Context) {
	// get the last two parts of the URL
	x := strings.Split(c.Request.URL.String(), `/`)
	// construct filename of desired package
	filename := x[len(x)-2] + "." + x[len(x)-1] + ".nupkg"
	// Debug Tracking
	log.Println("Serving Package", filename)

	// Loop through packages to find the one we need
	for _, p := range repo.packages {
		if p.Properties.ID == x[len(x)-2] && p.Properties.Version == x[len(x)-1] {
			// Set header to fix filename on client side
			c.Header("Cache-Control", "max-age=3600")
			c.Header("Content-Disposition", `filename=`+p.filename)
			// Serve up the file
			b, err := ioutil.ReadFile(filepath.Join(repo.rootDIR, p.Properties.ID, p.Properties.Version, p.filename))
			if err != nil {
				c.AbortWithStatus(500)
			}
			c.Data(200, "binary/octet-stream", b)
		}
	}
}

type profileForm struct {
	NupkgFile *multipart.FileHeader `form:"package" binding:"required"`
}

func putPackage(c *gin.Context) {
	// in this case proper binding will be automatically selected
	var form profileForm
	if err := c.ShouldBind(&form); err != nil {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	// f, err := form.NupkgFile.Open()
	// if err != nil {
	// 	c.AbortWithError(500, err)
	// }

	// err := c.SaveUploadedFile(form.NupkgFile, form.NupkgFile.Filename)
	// if err != nil {
	// 	c.String(http.StatusInternalServerError, "unknown error")
	// 	return
	// }

	// db.Save(&form)

	c.String(http.StatusOK, "ok")
}

func putPackage2(w http.ResponseWriter, r *http.Request) {

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
	packagePath := filepath.Join(cfg.RepoDIR, strings.ToLower(nsf.Meta.ID), nsf.Meta.Version)
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
