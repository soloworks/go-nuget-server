package main

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	nuspec "github.com/soloworks/go-nuspec"
)

// NugetServiceCollection used by NugetService
type NugetServiceCollection struct {
	Href  string `xml:"href,attr"`
	Title string `xml:"atom:title"`
}

// NugetService returned from a root Nuget request
type NugetService struct {
	XMLName   xml.Name `xml:"service"`
	XMLBase   string   `xml:"xml:base,attr"`
	XMLNs     string   `xml:"xmlns,attr"`
	XMLNsA    string   `xml:"xmlns:atom,attr"`
	Workspace struct {
		Title      string                   `xml:"atom:title"`
		Collection []NugetServiceCollection `xml:"collection"`
	} `xml:"workspace"`
}

// NewNugetService returns a populated skeleton for a root Nuget request (/)
func NewNugetService(baseURL string) *NugetService {

	ns := NugetService{}
	// Set Default Values
	ns.Workspace.Title = "Default"
	ns.XMLBase = baseURL
	ns.XMLNs = "http://www.w3.org/2007/app"
	ns.XMLNsA = "http://www.w3.org/2005/Atom"
	ns.Workspace.Collection = append(ns.Workspace.Collection, NugetServiceCollection{Href: "Packages", Title: "Packages"})
	ns.Workspace.Collection = append(ns.Workspace.Collection, NugetServiceCollection{Href: "Screenshots", Title: "Screenshots"})

	return &ns
}

// ToBytes exports structure as byte array
func (ns *NugetService) ToBytes() []byte {
	var b bytes.Buffer
	// Unmarshal into XML
	output, err := xml.MarshalIndent(ns, "  ", "    ")
	if err != nil {
	}
	b.WriteString(xml.Header)
	b.Write(output)
	return b.Bytes()

}

// NugetFeed represents the XML of a NugetFeed response
type NugetFeed struct {
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
	} `xml:"link"`
	Packages []*NugetPackageEntry
}

// NewNugetFeed returns a populated skeleton for a Nuget Packages request (/Packages)
func NewNugetFeed(title string, baseURL string) *NugetFeed {

	nf := NugetFeed{}
	// Set Feed Values
	nf.XMLBase = baseURL
	nf.XMLNs = "http://www.w3.org/2005/Atom"
	nf.XMLNsD = "http://schemas.microsoft.com/ado/2007/08/dataservices"
	nf.XMLNsM = "http://schemas.microsoft.com/ado/2007/08/dataservices/metadata"
	nf.ID = baseURL + title
	nf.Title.Text = title
	nf.Title.Type = "text"
	nf.Updated = time.Now().Format(zuluTimeLayout)
	nf.Link.Rel = "self"
	nf.Link.Title = title
	nf.Link.Href = title

	return &nf
}

// ToBytes exports structure as byte array
func (nf *NugetFeed) ToBytes() []byte {
	var b bytes.Buffer
	// Unmarshal into XML
	output, err := xml.MarshalIndent(nf, "  ", "    ")
	// Break XML Encoding to match Nuget server output
	output = bytes.ReplaceAll(output, []byte("&#39;"), []byte("'"))
	if err != nil {
	}
	// Self-Close any empty XML elements (NuGet client is broken and requires this on some)
	// This assumes Indented Marshalling above, non Indented will break XML
	// Break XML Encoding to match Nuget server output
	for bytes.Contains(output, []byte(`></`)) {
		i := bytes.Index(output, []byte(`></`))
		j := bytes.Index(output[i+1:], []byte(`>`))
		output = append(output[:i], append([]byte(` /`), output[i+j+1:]...)...)
	}

	// Replace http://hosturl/ with fully qualified urls
	output = bytes.ReplaceAll(output, []byte("http://hosturl/"), []byte(server.URL.String()))

	// Write the XML Header
	b.WriteString(xml.Header)
	b.Write(output)
	return b.Bytes()

}

// NugetPackageLink is used in NugetPackage
type NugetPackageLink struct {
	Rel   string `xml:"rel,attr"`
	Title string `xml:"title,attr"`
	Type  string `xml:"type,attr,omitempty"`
	Href  string `xml:"href,attr"`
}

// NugetPackageEntry is a single entry in a Nuget Feed
type NugetPackageEntry struct {
	PackageID      string   `xml:"-"`
	PackageVersion string   `xml:"-"`
	XMLName        xml.Name `xml:"entry"`
	XMLBase        string   `xml:"xml:base,attr,omitempty"`
	XMLNs          string   `xml:"xmlns,attr,omitempty"`
	XMLNsD         string   `xml:"xmlns:d,attr,omitempty"`
	XMLNsM         string   `xml:"xmlns:m,attr,omitempty"`
	ID             string   `xml:"id"`
	Category       struct {
		Term   string `xml:"term,attr"`
		Scheme string `xml:"scheme,attr"`
	} `xml:"category"`
	Link  []NugetPackageLink `xml:"link"`
	Title struct {
		Text string `xml:",chardata"`
		Type string `xml:"type,attr"`
	} `xml:"title"`
	Summary struct {
		Text string `xml:",chardata"`
		Type string `xml:"type,attr"`
	} `xml:"summary"`
	Updated string `xml:"updated"`
	Author  struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Content struct {
		Type string `xml:"type,attr"`
		Src  string `xml:"src,attr"`
	} `xml:"content"`
	Properties struct {
		ID          string `xml:"d:Id"`
		Version     string `xml:"d:Version"`
		VersionNorm string `xml:"d:NormalizedVersion"`
		Copyright   struct {
			Value string `xml:",chardata"`
			Null  bool   `xml:"m:null,attr"`
		} `xml:"d:Copyright"`
		Created struct {
			Value string `xml:",chardata"`
			Type  string `xml:"m:type,attr"`
		} `xml:"d:Created"`
		Dependencies  string `xml:"d:Dependencies"`
		Description   string `xml:"d:Description"`
		DownloadCount struct {
			Value int    `xml:",chardata"`
			Type  string `xml:"m:type,attr"`
		} `xml:"d:DownloadCount"`
		GalleryDetailsURL string `xml:"d:GalleryDetailsUrl"`
		IconURL           string `xml:"d:IconUrl"`
		IsLatestVersion   struct {
			Value bool   `xml:",chardata"`
			Type  string `xml:"m:type,attr"`
		} `xml:"d:IsLatestVersion"`
		IsAbsoluteLatestVersion struct {
			Value bool   `xml:",chardata"`
			Type  string `xml:"m:type,attr"`
		} `xml:"d:IsAbsoluteLatestVersion"`
		LastEdited struct {
			Value string `xml:",chardata"`
			Type  string `xml:"m:type,attr"`
		} `xml:"d:LastEdited"`
		Published struct {
			Value string `xml:",chardata"`
			Type  string `xml:"m:type,attr"`
		} `xml:"d:Published"`
		LicenseURL struct {
			Value string `xml:",chardata"`
			Null  bool   `xml:"m:null,attr"`
		} `xml:"d:LicenseUrl"`
		LicenseNames struct {
			Value string `xml:",chardata"`
			Null  bool   `xml:"m:null,attr"`
		} `xml:"d:LicenseNames"`
		LicenseReportURL struct {
			Value string `xml:",chardata"`
			Null  bool   `xml:"m:null,attr"`
		} `xml:"d:LicenseReportUrl"`
		PackageHash          string `xml:"d:PackageHash"`
		PackageHashAlgorithm string `xml:"d:PackageHashAlgorithm"`
		PackageSize          struct {
			Value int    `xml:",chardata"`
			Type  string `xml:"m:type,attr"`
		} `xml:"d:PackageSize"`
		ProjectURL   string `xml:"d:ProjectUrl"`
		ReleaseNotes struct {
			Value string `xml:",chardata"`
			Null  bool   `xml:"m:null,attr"`
		} `xml:"d:ReleaseNotes"`
		ReportAbuseURL           string `xml:"d:ReportAbuseUrl"`
		RequireLicenseAcceptance struct {
			Value bool   `xml:",chardata"`
			Type  string `xml:"m:type,attr"`
		} `xml:"d:RequireLicenseAcceptance"`
		Tags                 string `xml:"d:Tags"`
		Title                string `xml:"d:Title"`
		VersionDownloadCount struct {
			Value int    `xml:",chardata"`
			Type  string `xml:"m:type,attr"`
		} `xml:"d:VersionDownloadCount"`
		IsPrerelease struct {
			Value bool   `xml:",chardata"`
			Type  string `xml:"m:type,attr"`
		} `xml:"d:IsPrerelease"`
		MinClientVersion struct {
			Value string `xml:",chardata"`
			Null  bool   `xml:"m:null,attr"`
		} `xml:"d:MinClientVersion"`
		Language string `xml:"d:Language"`
	} `xml:"m:properties"`
}

// NewNugetPackageEntry returns a semi populated struct for a Nuget Packages Entry
func NewNugetPackageEntry(nsf *nuspec.File) *NugetPackageEntry {
	// Create new entry
	e := NugetPackageEntry{}
	// Values for ease of reference & searching
	e.PackageID = nsf.Meta.ID
	e.PackageVersion = nsf.Meta.Version
	// Set Defaults
	e.Category.Term = `MyGet.V2FeedPackage`
	e.Category.Scheme = `http://schemas.microsoft.com/ado/2007/08/dataservices/scheme`
	e.Link = append(e.Link, NugetPackageLink{
		Rel:   "edit",
		Title: "V2FeedPackage",
		Href:  "Packages(Id='" + nsf.Meta.ID + `',Version='` + nsf.Meta.Version + `')`,
	})
	e.Link = append(e.Link, NugetPackageLink{
		Rel:   "http://schemas.microsoft.com/ado/2007/08/dataservices/related/Screenshots",
		Type:  "application/atom+xml;type=feed",
		Title: "Screenshots",
		Href:  "Packages(Id='" + nsf.Meta.ID + `',Version='` + nsf.Meta.Version + `')/Screenshots`,
	})
	e.Link = append(e.Link, NugetPackageLink{
		Rel:   "edit-media",
		Title: "V2FeedPackage",
		Href:  "Packages(Id='" + nsf.Meta.ID + `',Version='` + nsf.Meta.Version + `')/$value`,
	})

	// Match and set main values
	e.ID = "http://hosturl/" + "Packages(Id='" + nsf.Meta.ID + `',Version='` + nsf.Meta.Version + `')`
	e.Title.Text = nsf.Meta.Title
	e.Title.Type = "Text"
	e.Summary.Text = nsf.Meta.Summary
	e.Summary.Type = "Text"
	e.Author.Name = nsf.Meta.Authors
	e.Content.Type = "binary/octet-stream"
	e.Content.Src = "http://hosturl/" + `nupkg/` + nsf.Meta.ID + `/` + nsf.Meta.Version + ``

	// Match and set property values
	e.Properties.ID = nsf.Meta.ID
	e.Properties.Version = nsf.Meta.Version
	e.Properties.VersionNorm = nsf.Meta.Version
	e.Properties.Copyright.Value = nsf.Meta.Copyright
	if e.Properties.Copyright.Value == "" {
		e.Properties.Copyright.Null = true
	}
	e.Properties.Description = nsf.Meta.Description
	e.Properties.GalleryDetailsURL = nsf.Meta.ProjectURL
	e.Properties.IconURL = nsf.Meta.IconURL
	e.Properties.IsLatestVersion.Value = true
	e.Properties.IsLatestVersion.Type = "Edm.Boolean"
	e.Properties.IsAbsoluteLatestVersion.Value = true
	e.Properties.IsAbsoluteLatestVersion.Type = "Edm.Boolean"
	e.Properties.ProjectURL = nsf.Meta.ProjectURL
	if e.Properties.ReleaseNotes.Value == "" {
		e.Properties.ReleaseNotes.Null = true
	}
	if e.Properties.LicenseURL.Value == "" {
		e.Properties.LicenseURL.Null = true
	}
	if e.Properties.LicenseNames.Value == "" {
		e.Properties.LicenseNames.Null = true
	}
	if e.Properties.LicenseReportURL.Value == "" {
		e.Properties.LicenseReportURL.Null = true
	}
	e.Properties.ReportAbuseURL = "http://soloworks.co.uk/"
	e.Properties.Tags = nsf.Meta.Tags
	e.Properties.Title = nsf.Meta.Title
	e.Properties.Language = "en-US"
	if e.Properties.MinClientVersion.Value == "" {
		e.Properties.MinClientVersion.Null = true
	}

	// Set other values
	e.Properties.Created.Type = "Edm.DateTime"
	e.Properties.DownloadCount.Type = "Edm.Int32"
	e.Properties.IsPrerelease.Type = "Edm.Boolean"
	e.Properties.LastEdited.Type = "Edm.DateTime"
	e.Properties.Published.Type = "Edm.DateTime"
	e.Properties.RequireLicenseAcceptance.Type = "Edm.Boolean"
	e.Properties.VersionDownloadCount.Type = "Edm.Int32"

	// Replace http://content/ with internal full URLs
	// pkgURL := "http://hosturl/" + "files/" + e.Properties.ID + `/` + e.Properties.Version + `/content/`
	// e.Properties.IconURL = strings.ReplaceAll(e.Properties.IconURL, "http://content/", pkgURL)
	// e.Properties.Description = strings.ReplaceAll(e.Properties.Description, "http://content/", pkgURL)

	// Return skeleton
	return &e
}

// Filename returns the logical filename for this package
func (npe *NugetPackageEntry) Filename() string {
	return npe.Properties.ID + "." + npe.Properties.Version + ".nupkg"
}

// ToBytes exports structure as byte array
func (npe *NugetPackageEntry) ToBytes() []byte {

	// If this is used then this is the root object of the feed
	npe.XMLBase = server.URL.String()
	npe.XMLNs = "http://www.w3.org/2005/Atom"
	npe.XMLNsD = "http://schemas.microsoft.com/ado/2007/08/dataservices"
	npe.XMLNsM = "http://schemas.microsoft.com/ado/2007/08/dataservices/metadata"

	var b bytes.Buffer
	// Unmarshal into XML
	output, err := xml.MarshalIndent(npe, "  ", "    ")
	if err != nil {

	}
	// Break XML Encoding to match Nuget server output
	output = bytes.ReplaceAll(output, []byte("&#39;"), []byte("'"))
	// Self-Close any empty XML elements (NuGet client is broken and requires this on some)
	// This assumes Indented Marshalling above, non Indented will break XML
	// Break XML Encoding to match Nuget server output
	for bytes.Contains(output, []byte(`></`)) {
		i := bytes.Index(output, []byte(`></`))
		j := bytes.Index(output[i+1:], []byte(`>`))
		output = append(output[:i], append([]byte(` /`), output[i+j+1:]...)...)
	}

	// Replace http://hosturl/ with fully qualified urls
	output = bytes.ReplaceAll(output, []byte("http://hosturl/"), []byte(server.URL.String()))

	// Write the XML Header
	b.WriteString(xml.Header)
	b.Write(output)
	return b.Bytes()
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
