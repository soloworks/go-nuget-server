package main

import (
	"encoding/xml"
	"log"
	"net/http"
	"strings"
)

func main() {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Routing: " + r.URL.String())
		switch {
		case r.URL.String() == `/q-sys-plugins/`:
			pathRoot(w, r)

		case strings.HasPrefix(r.URL.String(), `/q-sys-plugins/Packages`):
			pathPackages(w, r)
		}
	})

	log.Println("Serving on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// XML structure for root XML service description returned at '/'
type nugetServiceCollection struct {
	Href  string `xml:"href,attr"`
	Title string `xml:"atom:title"`
}
type nugetService struct {
	XMLName   xml.Name `xml:"service"`
	XMLBase   string   `xml:"xml:base,attr"`
	XMLNs     string   `xml:"xmlns,attr"`
	XMLNsA    string   `xml:"xmlns:atom,attr"`
	Workspace struct {
		Title      string                   `xml:"atom:title"`
		Collection []nugetServiceCollection `xml:"collection"`
	} `xml:"workspace"`
}

func pathRoot(w http.ResponseWriter, r *http.Request) {
	log.Println("Serving Root")

	// Set Headers
	w.Header().Set("Content-Type", "application/xml;charset=utf-8")

	// Create a new Service Struct
	ns := nugetService{}

	// Set Default Values
	ns.Workspace.Title = "Default"
	ns.XMLBase = "http://" + r.Host + r.RequestURI
	ns.XMLNs = "http://www.w3.org/2007/app"
	ns.XMLNsA = "http://www.w3.org/2005/Atom"
	ns.Workspace.Collection = append(ns.Workspace.Collection, nugetServiceCollection{Href: "Packages", Title: "Packages"})
	ns.Workspace.Collection = append(ns.Workspace.Collection, nugetServiceCollection{Href: "Screenshots", Title: "Screenshots"})

	// Unmarshal into XML
	output, err := xml.MarshalIndent(ns, "  ", "    ")
	if err != nil {
	}
	w.Write([]byte(xml.Header))
	w.Write(output)
}

type nugetFeed struct {
	XMLName xml.Name `xml:"feed"`
	XMLBase string   `xml:"xml:base,attr"`
	XMLNs   string   `xml:"xmlns,attr"`
	XMLNsD  string   `xml:"xmlns:d,attr"`
	XMLNsM  string   `xml:"xmlns:m,attr"`
	ID      string   `xml:"id"`
	Title   struct {
		Text string `xml:",chardata"`
		Type string `xml:"type,attr"`
	} `xml:"title"`
	Updated string `xml:"updated"`
	Link    struct {
		Rel   string `xml:"rel,attr"`
		Title string `xml:"title,attr"`
		Href  string `xml:"href,attr"`
	}
}

type nugetFeedEntry struct {
	Author struct {
		Name string `xml:"name"`
	} `xml:"author"`
}

func pathPackages(w http.ResponseWriter, r *http.Request) {
	log.Println("Serving Root")

	// Set Headers
	w.Header().Set("Content-Type", "application/xml;charset=utf-8")

	// Create a new Service Struct
	nf := nugetFeed{}

	// Set Feed Values
	nf.XMLBase = "http://" + r.Host + r.RequestURI // Should not have Packages at the end
	nf.XMLNs = "http://www.w3.org/2005/Atom"
	nf.XMLNsD = "http://schemas.microsoft.com/ado/2007/08/dataservices"
	nf.XMLNsM = "http://schemas.microsoft.com/ado/2007/08/dataservices/metadata"
	nf.ID = nf.XMLBase
	nf.Title.Text = "Packages"
	nf.Title.Type = "text"
	//nf.Updated = "2018-07-27T23:10:35Z"
	nf.Link.Rel = "self"
	nf.Link.Title = "Packages"
	nf.Link.Href = "Packages"

	// Process all packages

	// Unmarshal into XML
	output, err := xml.MarshalIndent(nf, "  ", "    ")
	if err != nil {
	}
	w.Write([]byte(xml.Header))
	w.Write(output)
}
