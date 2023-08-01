# Mongo Storage for [OAuth 2.0](https://github.com/go-oauth2/oauth2)

[![Build][Build-Status-Image]][Build-Status-Url] [![Codecov][codecov-image]][codecov-url] [![ReportCard][reportcard-image]][reportcard-url] [![GoDoc][godoc-image]][godoc-url] [![License][license-image]][license-url]

## Install

``` bash
$ go get -u -v gopkg.in/go-oauth2/mongo.v3
```

## Usage

``` go
import(
    "github.com/go-oauth2/oauth2/v4/manage"
    "github.com/go-oauth2/oauth2/v4/server"
    mongo "gopkg.in/go-oauth2/mongo.v3"    
)

func main(){
    manager := manage.NewDefaultManager()

    /*
	* only for a MongoDB replicaSet deployment
    * Using a replicaSet is recommended as it allows for MongoDB's native support for transactions
    **/
	// mongoConf := mongo.NewConfigReplicaSet(
	// 	"mongodb://localhost:27017,localhost:28017,localhost:29017/?replicaSet=myReplicaSet",
	// 	"oauth2",
	// )

	// set connectionTimeout(7s) and the requestsTimeout(5s) // is optional
	storeConfigs := mongo.NewStoreConfig(7, 5)

    /*
	* for a single mongoDB node
	* if the oauth2 service is deployed with more than one instance
	* each mongoConf should have unique serviceName
    **/
	mongoConf := mongo.NewConfigNonReplicaSet(
		"mongodb://127.0.0.1:27017",
		"oauth2",   // database name
		"admin",    // username to authenticate with db
		"password", // password to authenticate with db
		"serviceName",
	)

	// use mongodb token store
	manager.MapTokenStorage(
		mongo.NewTokenStore(mongoConf, storeConfigs), // with timeout
		// mongo.NewTokenStore(mongoConf), // no timeout
	)

	clientStore := mongo.NewClientStore(mongoConf, storeConfigs) // with timeout
	// clientStore := mongo.NewClientStore(mongoConf) // no timeout

	manager.MapClientStorage(clientStore)

	// register a service
	clientStore.Create(&models.Client{
		ID:     idvar,
		Secret: secretvar,
		Domain: domainvar,
		UserID: "frontend",
	})

	// register a second service
	clientStore.Create(&models.Client{
		ID:     idPreorder,
		Secret: secretPreorder,
		Domain: domainPreorder,
		UserID: "prePost",
	})

	srv := server.NewServer(server.NewConfig(), manager)

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
