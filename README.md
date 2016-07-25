MongoDB Storage for OAuth2
==========================

[![GoDoc](https://godoc.org/github.com/go-oauth2/mongo?status.svg)](https://godoc.org/github.com/go-oauth2/mongo)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-oauth2/mongo)](https://goreportcard.com/report/github.com/go-oauth2/mongo)

Install
-------

``` bash
$ go get -u -v github.com/go-oauth2/mongo
```

Usage
-----

``` go
package main

import (
	"github.com/go-oauth2/mongo"
	"gopkg.in/oauth2.v3/manage"
)

func main() {
	manager := manage.NewDefaultManager()
	// use mongodb token store
    mcfg := mongo.NewConfig("mongodb://admin:123456@192.168.33.70:27017", "oauth2")
	manager.MustTokenStorage(mongo.NewTokenStore(mcfg))

	// ...
}
```

License
-------

```
Copyright (c) 2016, OAuth 2.0
All rights reserved.
```