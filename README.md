# go-nuget-server

[![MIT license](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0.en.html)
[![LinkedIn](https://img.shields.io/badge/Contact-LinkedIn-blue)](https://www.linkedin.com/company/soloworkslondon/)

A minimal Nuget HTTP(s) server written in Go, primarily developed to serve the Q-Sys plugin platform.

Tested against:

- (client) NuGet/2.14.0.832 built into QSC Q-Sys Designer 8.1.1
- (cli tool) nuget.exe (Microsoft: 5.2.0.6090)
- (cli tool) go-nuget

## Notes

Nuget is strange. It doesn't seem to respect it's own protocols and APIs.

Irresepective of supplied paths, it will still occasionally try to find static files in `/F/<yoururl>/api/v2/browse/`.

Documentation states `<iconURL>` is depreciated for `<icon>` which can look for files in package instead of over http. However trying to pack with latest Nuget.exe fails on this against the schema.

## Acknowledgements
