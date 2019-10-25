package main

import (
	"context"
	"log"
	"path"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	firebase "firebase.google.com/go"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
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
	fs.bucket = sc.Bucket(s.config.FileStore.BucketName)
	if err != nil {
		return err
	}

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

func (fs *fileStoreGCP) StorePackage(pkg []byte) error {

	// Extract files
	nsf, files, err := extractPackage(pkg)
	if err != nil {
		return err
	}

	// Generate local variables for ease
	pkgRef := nsf.Meta.ID + "." + nsf.Meta.Version
	pkgFileName := pkgRef + ".nupkg"                   // Package File Name
	pkgDir := path.Join(nsf.Meta.ID, nsf.Meta.Version) // Package Directory Name

	// Save Package
	wc := fs.bucket.Object(path.Join(pkgDir, pkgFileName)).NewWriter(fs.ctx)
	wc.ContentType = "application/octet-stream"
	if _, err := wc.Write(pkg); err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	// Save Files
	for name, content := range files {
		wc := fs.bucket.Object(path.Join(pkgDir, name)).NewWriter(fs.ctx)
		wc.ContentType = "application/octet-stream"
		if _, err := wc.Write(content); err != nil {
			return err
		}
		if err := wc.Close(); err != nil {
			return err
		}
	}

	// Make a new Package Entry and add it to the Database
	npe := NewNugetPackageEntry(nsf)
	if _, err := fs.firestore.Collection("Packages").Doc(pkgRef).Set(fs.ctx, npe); err != nil {
		return err
	}

	// Return
	return nil
}

func (fs *fileStoreGCP) GetPackage(id string, ver string) (*NugetPackageEntry, error) {

	// New array to pass back
	var pkg *NugetPackageEntry

	d, err := fs.firestore.Collection("Packages").Doc(id + "." + ver).Get(fs.ctx)
	if err != nil {
		return nil, err
	}

	if err := d.DataTo(&pkg); err != nil {
		return nil, err
	}

	return pkg, nil
}

func (fs *fileStoreGCP) GetPackages(id string) ([]*NugetPackageEntry, error) {

	// New array to pass back
	var pkgs []*NugetPackageEntry
	var iter *firestore.DocumentIterator

	if id == "" {
		iter = fs.firestore.Collection("Packages").Documents(fs.ctx)
	} else {
		iter = fs.firestore.Collection("Packages").Where("PackageID", "==", id).Documents(fs.ctx)
	}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var p *NugetPackageEntry
		if err := doc.DataTo(&p); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}
