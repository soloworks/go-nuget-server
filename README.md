# go-nuget-server

[![MIT license](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0.en.html)
[![LinkedIn](https://img.shields.io/badge/Contact-LinkedIn-blue)](https://www.linkedin.com/company/soloworkslondon/)
![](https://github.com/soloworks/go-nuget-server/workflows/Build/badge.svg)

A minimal Nuget HTTP(s) server written in Go, primarily developed to serve the Q-Sys plugin platform.

Tested against:

- (client) NuGet/2.14.0.832 built into QSC Q-Sys Designer 8.1.1
- (cli tool) nuget.exe (Microsoft: 5.2.0.6090)
- (cli tool) go-nuget <https://github.com/soloworks/go-nuget/>

## Getting Started

This server is a Lightweight implementation, tested against the above Nuget client. Development started as a file system based store (using local storage), but this was abandoned for a GCP based system using Firebase and Google Run.

All file and database functionality is abstracted into a FileStore interface which can be re-implemented as any other storage/database combination as desired. Just add a new switch, new filestore implementation and code away.

Security is APIKey based only. Having no keys present will result in an open server, any ReadWrite keys present will require one to write but leave free read access. Any ReadOnly keys present will lock down all requests to require an API key. For Firebase this requires an entry in a collection called `Nuget-APIKeys` where the document name is the key and has at least one field called `Access` which can have the values `ReadOnly|ReadWrite`. 

## Notes

Nuget is strange. It doesn't seem to respect it's own protocols and APIs.

Irresepective of supplied paths, it will still occasionally try to find static files in `/F/<yoururl>/api/v2/browse/`.

Documentation states `<iconURL>` is depreciated for `<icon>` which can look for files in package instead of over http. However trying to pack with latest Nuget.exe fails on this against the schema.

## Acknowledgements
