# Mongo Storage for [OAuth 2.0](https://github.com/go-oauth2/oauth2)

[![Build][Build-Status-Image]][Build-Status-Url] [![Codecov][codecov-image]][codecov-url] [![ReportCard][reportcard-image]][reportcard-url] [![GoDoc][godoc-image]][godoc-url] [![License][license-image]][license-url]

## Install

``` bash
$ go get -u -v gopkg.in/go-oauth2/mongo.v3
```

## Usage

``` go
package main

import (
	"gopkg.in/go-oauth2/mongo.v3"
	"gopkg.in/oauth2.v3/manage"
)

func main() {
	manager := manage.NewDefaultManager()

	// use mongodb token store
	manager.MapTokenStorage(
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

[Build-Status-Url]: https://travis-ci.org/go-oauth2/mongo
[Build-Status-Image]: https://travis-ci.org/go-oauth2/mongo.svg?branch=master
[codecov-url]: https://codecov.io/gh/go-oauth2/mongo
[codecov-image]: https://codecov.io/gh/go-oauth2/mongo/branch/master/graph/badge.svg
[reportcard-url]: https://goreportcard.com/report/gopkg.in/go-oauth2/mongo.v3
[reportcard-image]: https://goreportcard.com/badge/gopkg.in/go-oauth2/mongo.v3
[godoc-url]: https://godoc.org/gopkg.in/go-oauth2/mongo.v3
[godoc-image]: https://godoc.org/gopkg.in/go-oauth2/mongo.v3?status.svg
[license-url]: http://opensource.org/licenses/MIT
[license-image]: https://img.shields.io/npm/l/express.svg
