# MongoDB Storage for OAuth 2.0

> Based on the mongodb token storage

[![License][License-Image]][License-Url] 
[![ReportCard][ReportCard-Image]][ReportCard-Url] 
[![GoDoc][GoDoc-Image]][GoDoc-Url]

## Install

``` bash
$ go get -u -v gopkg.in/go-oauth2/mongo.v1
```

## Usage

``` go
package main

import (
	"gopkg.in/go-oauth2/mongo.v1"
	"gopkg.in/oauth2.v3/manage"
)

func main() {
	manager := manage.NewDefaultManager()
	// use mongodb token store
	manager.MustTokenStorage(
		mongo.NewTokenStore(mongo.NewConfig(
			"mongodb://127.0.0.1:27017",
			"oauth2",
		)),
	)
	// ...
}
```

## MIT License

```
Copyright (c) 2016 Lyric
```

[License-Url]: http://opensource.org/licenses/MIT
[License-Image]: https://img.shields.io/npm/l/express.svg
[ReportCard-Url]: https://goreportcard.com/report/github.com/go-oauth2/mongo
[ReportCard-Image]: https://goreportcard.com/badge/github.com/go-oauth2/mongo
[GoDoc-Url]: https://godoc.org/github.com/go-oauth2/mongo
[GoDoc-Image]: https://godoc.org/github.com/go-oauth2/mongo?status.svg