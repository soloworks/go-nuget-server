package main

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"log"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	firebase "firebase.google.com/go"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type fileStoreGCP struct {
	ctx       context.Context
	creds     *google.Credentials
	bucket    *storage.BucketHandle
	firestore *firestore.Client
}

func (fs *fileStoreGCP) Init(s *Server) error {

	// Had to add this to avoid compiler errors...
	var err error

	// Set Google Background Context
	fs.ctx = context.Background()

	// Connect to Storage Bucket specified in config
	sc, err := storage.NewClient(fs.ctx)
	if err != nil {
		return err
	}
	fs.bucket = sc.Bucket(s.config.FileStore.BucketName)

	// Open connection to Firestore
	conf := &firebase.Config{ProjectID: s.config.FileStore.ProjectID}
	app, err := firebase.NewApp(fs.ctx, conf)
	if err != nil {
		log.Fatalln(err)
	}

	fs.firestore, err = app.Firestore(fs.ctx)
	if err != nil {
		log.Fatalln(err)
	}
	return nil
}

func (fs *fileStoreGCP) StorePackage(pkg []byte) (bool, error) {

	// Extract files
	nsf, files, err := extractPackage(pkg)
	if err != nil {
		return false, err
	}

	// Generate local variables for ease
	pkgRef := nsf.Meta.ID + "." + nsf.Meta.Version
	pkgFileName := pkgRef + ".nupkg"                   // Package File Name
	pkgDir := path.Join(nsf.Meta.ID, nsf.Meta.Version) // Package Directory Name

	// Check to see if package already exists
	d, err := fs.firestore.Collection("Nuget-Packages").Doc(pkgRef).Get(fs.ctx)
	if err != nil && grpc.Code(err) != codes.NotFound {
		return false, err
	}
	if d.Exists() {
		return true, nil
	}

	// Save Package
	wc := fs.bucket.Object(path.Join(pkgDir, pkgFileName)).NewWriter(fs.ctx)
	wc.ContentType = "application/octet-stream"
	if _, err := wc.Write(pkg); err != nil {
		return false, err
	}
	if err := wc.Close(); err != nil {
		return false, err
	}

	// Save Files
	for name, content := range files {
		wc := fs.bucket.Object(path.Join(pkgDir, name)).NewWriter(fs.ctx)
		wc.ContentType = "application/octet-stream"
		if _, err := wc.Write(content); err != nil {
			return false, err
		}
		if err := wc.Close(); err != nil {
			return false, err
		}

	}

	// Make a new Package Entry
	npe := NewNugetPackageEntry(nsf)

	// Populate additional time values
	npe.Properties.Created.Value = time.Now().Format(zuluTimeLayout)
	npe.Properties.LastEdited.Value = time.Now().Format(zuluTimeLayout)
	npe.Properties.Published.Value = time.Now().Format(zuluTimeLayout)
	npe.Updated = time.Now().Format(zuluTimeLayout)

	// Populate additional package values
	h := sha512.Sum512(pkg)
	npe.Properties.PackageHash = hex.EncodeToString(h[:])
	npe.Properties.PackageHashAlgorithm = `SHA512`
	npe.Properties.PackageSize.Value = len(pkg)
	npe.Properties.PackageSize.Type = "Edm.Int64"

	// Save to Firestore
	if _, err := fs.firestore.Collection("Nuget-Packages").Doc(pkgRef).Set(fs.ctx, npe); err != nil {
		return false, err
	}

	// Local Extras object
	pe := &packagesExtra{}

	// Cycle through all packages with this ID to get the latest version
	iter := fs.firestore.Collection("Nuget-Packages").Where("Properties.ID", "==", npe.Properties.ID).Documents(fs.ctx)
	// Cycle Iterator
	for {
		d, err = iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return false, err
		}
		// Marshall into structure
		var npe *NugetPackageEntry
		if err := d.DataTo(&npe); err != nil {
			return false, err
		}
		// Check against latest and overrite if higher
		if npe.Properties.Version > pe.Latest {
			pe.Latest = npe.Properties.Version
		}
	}

	// Ensure Extras is created for this id
	if _, err := fs.firestore.Collection("Nuget-Packages-Extra").Doc(npe.Properties.ID).Set(fs.ctx,
		pe,
		firestore.Merge([]string{"Latest"}),
	); err != nil {
		return false, err
	}
	// Return
	return false, nil
}

type packagesExtra struct {
	Downloads int
	Latest    string
}

func (fs *fileStoreGCP) getPackageExtras(id string) (*packagesExtra, error) {

	// Get additional data - Download counts and check if latest version
	// Fetch the additional data document for this ID
	d, err := fs.firestore.Collection("Nuget-Packages-Extra").Doc(id).Get(fs.ctx)
	if err != nil {
		return nil, err
	}
	if d.Exists() {
		// Marshall into structure
		var pe *packagesExtra
		if err := d.DataTo(&pe); err != nil {
			return nil, err
		}
		return pe, nil
	}
	return nil, errors.New("Can't Find Nuget-Package-Extra")
}

func (fs *fileStoreGCP) GetPackageEntry(id string, ver string) (*NugetPackageEntry, error) {

	// Fetch this document
	d, err := fs.firestore.Collection("Nuget-Packages").Doc(id + "." + ver).Get(fs.ctx)
	if err != nil {
		return nil, err
	}

	// Marshall into structure
	var npe *NugetPackageEntry
	if err := d.DataTo(&npe); err != nil {
		return nil, err
	}

	pe, err := fs.getPackageExtras(id)
	if err != nil {
		return nil, err
	}
	// Get download count for this id all versions
	npe.Properties.DownloadCount.Value = pe.Downloads
	// Get latest version and compare to this
	npe.Properties.IsLatestVersion.Value = pe.Latest == ver
	npe.Properties.IsAbsoluteLatestVersion.Value = pe.Latest == ver

	// TODO: Returns 500 error when no matching package - should return 404
	return npe, nil
}

func (fs *fileStoreGCP) GetPackageFeedEntries(id string, startAfter string, max int) ([]*NugetPackageEntry, error) {

	// Create new empty feed
	var f []*NugetPackageEntry
	// Create new itterator
	var iter *firestore.DocumentIterator
	// Create map of extra details so only looked up once per id
	extras := make(map[string]*packagesExtra)
	// Populate Itterator
	if startAfter != "" {
		// Get specific APIKey entry
		d, err := fs.firestore.Collection("Nuget-Packages").Doc(startAfter).Get(fs.ctx)
		if err != nil {
			return nil, err
		}
		iter = fs.firestore.Collection("Nuget-Packages").StartAfter(d).Limit(max).Documents(fs.ctx)
	} else if id != "" {
		iter = fs.firestore.Collection("Nuget-Packages").Limit(max).Where("Properties.IDLowerCase", "==", strings.ToLower(id)).Documents(fs.ctx)
	} else {
		iter = fs.firestore.Collection("Nuget-Packages").Limit(max).Documents(fs.ctx)
	}
	// Cycle Iterator
	for {
		// Get next
		doc, err := iter.Next()
		// If done, break from loop
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		// Cast document into structure
		var e *NugetPackageEntry
		if err := doc.DataTo(&e); err != nil {
			return nil, err
		}
		// Get extras if not in map already
		if _, ok := extras[e.Properties.ID]; !ok {
			extra, err := fs.getPackageExtras(e.Properties.ID)
			if err != nil {
				return nil, err
			}
			extras[e.Properties.ID] = extra
		}
		// Add extra details to entry
		e.Properties.DownloadCount.Value = extras[e.Properties.ID].Downloads
		e.Properties.IsLatestVersion.Value = extras[e.Properties.ID].Latest == e.Properties.Version
		e.Properties.IsAbsoluteLatestVersion.Value = extras[e.Properties.ID].Latest == e.Properties.Version
		// Add in to list
		f = append(f, e)
	}
	return f, nil
}

func (fs *fileStoreGCP) GetPackageFile(id string, ver string) ([]byte, string, error) {

	// Set the filename
	key := id + "." + ver

	// Get the file
	b, _, err := fs.GetFile(path.Join(id, ver, key+".nupkg"))
	if err != nil {
		return nil, "", err
	}

	// Increment this verson's download count
	_, err = fs.firestore.Collection("Nuget-Packages").Doc(key).Update(fs.ctx, []firestore.Update{
		{Path: "Properties.VersionDownloadCount.Value", Value: firestore.Increment(1)},
	})
	if err != nil {
		return nil, "", err
	}

	// Increment this ID's download count
	_, err = fs.firestore.Collection("Nuget-Packages-Extra").Doc(id).Update(fs.ctx, []firestore.Update{
		{Path: "Downloads", Value: firestore.Increment(1)},
	})
	if err != nil {
		return nil, "", err
	}

	// Return it
	return b, "binary/octet-stream", nil
}

func (fs *fileStoreGCP) GetFile(f string) ([]byte, string, error) {

	if strings.HasPrefix(f, `/`) {
		f = f[1:]
	}

	// Check for exact match
	obj := fs.bucket.Object(f)
	a, err := obj.Attrs(fs.ctx)
	if err == storage.ErrObjectNotExist {
		// Check for lowercase filename match (Due to zip file not keeping cases)
		d := path.Dir(f)
		fn := path.Base(f)
		fp := path.Join(d, strings.ToLower(fn))
		obj = fs.bucket.Object(fp)
		_, err = obj.Attrs(fs.ctx)
		if err == storage.ErrObjectNotExist {
			// ToDo: Full loop of directory contents on ToLower comparison of full
			// path looking for match
			return nil, "", ErrFileNotFound
		} else if err != nil {
			return nil, "", err
		}
	} else if err != nil {
		return nil, "", err
	}

	r, err := obj.NewReader(fs.ctx)
	if err != nil {
		return nil, "", err
	}
	defer r.Close()
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, "", err
	}

	return b, a.ContentType, nil
}

// FirestoreAPIKey represents a ApiKey as stored in Firebase
type FirestoreAPIKey struct {
	Reference string
	Access    string
}

func (fs *fileStoreGCP) GetAccessLevel(key string) (access, error) {

	// Set default variables
	var err error
	a := accessDenied
	var iter *firestore.DocumentIterator

	// Check for case where no ReadOnly keys are in place
	iter = fs.firestore.Collection("Nuget-APIKeys").Where("Access", "==", "ReadOnly").Documents(fs.ctx)
	_, err = iter.Next()
	// Attempt to advance to first in the list
	if err == iterator.Done {
		// No ReadWrite keys were found, default access becomes ReadOnly
		a = accessReadOnly
	} else if err != nil {
		// Another error happened, return no access and error
		return a, err
	}

	// Check for case where no keys are declared yet - dev mode
	iter = fs.firestore.Collection("Nuget-APIKeys").Documents(fs.ctx)
	_, err = iter.Next()
	// Attempt to advance to first in the list
	if err == iterator.Done {
		// No ReadWrite keys were found, access granted as server in dev mode
		return accessReadWrite, nil
	} else if err != nil {
		// Another error happened, return no access and error
		return a, err
	}

	// Get specific APIKey entry
	k := FirestoreAPIKey{}
	d, err := fs.firestore.Collection("Nuget-APIKeys").Doc(key).Get(fs.ctx)
	if err != nil {
		return a, nil
	}
	// Convert to local structure
	if err := d.DataTo(&k); err != nil {
		return a, nil
	}
	// Grant access if permission present on key
	switch k.Access {
	case "ReadWrite":
		a = accessReadWrite
	case "ReadOnly":
		a = accessReadOnly
	}
	// Deny access if not
	return a, nil
}
