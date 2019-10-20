# go-nuget-server

A minimal Nuget HTTP(s) server written in Go, primarily developed to serve the Q-Sys plugin platform

[![MIT license](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0.en.html)
[![LinkedIn](https://img.shields.io/badge/Contact-LinkedIn-blue)](https://www.linkedin.com/company/soloworkslondon/)

## Process

Useful post about file structure:
https://stackoverflow.com/questions/9642183/how-to-create-a-nuget-package-by-hand-without-nuget-exe-or-nuget-explorer

On startup, Q-Sys Designer sends two queries to the plugin host Url:

```
~UrlString~/
~UrlString~/Packages()
```

The () represent an empty set of filters - when empty, they yeild the same result as not being present

Results from the same queries applied to the Q-Sys Plugins Server

<https://qsysassets.myget.org/F/qsc-managed-plugins/>

```xml
<?xml version="1.0" encoding="utf-8"?>
<service xml:base="https://qsysassets.myget.org/F/qsc-managed-plugins/" xmlns="http://www.w3.org/2007/app" xmlns:atom="http://www.w3.org/2005/Atom">
    <workspace>
        <atom:title>Default</atom:title>
        <collection href="Packages">
            <atom:title>Packages</atom:title>
        </collection>
        <collection href="Screenshots">
            <atom:title>Screenshots</atom:title>
        </collection>
    </workspace>
</service>
```

<https://qsysassets.myget.org/F/qsc-managed-plugins/Packages()>

```
Contents of Q-Sys-Nuget-Packages.xml (47KB)
```

## Acknowledgements
