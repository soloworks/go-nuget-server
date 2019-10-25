package main

import (
	"context"
	"log"
	"path"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	firebase "firebase.google.com/go"
	"golang.org/x/oauth2/google"
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

	return nil, nil
}

func (fs *fileStoreGCP) GetPackages(id string) ([]*NugetPackageEntry, error) {

	return nil, nil
}
