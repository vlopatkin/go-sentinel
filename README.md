## go-sentinel

[![Build Status](https://travis-ci.org/arwx/go-sentinel.svg?branch=master)](https://travis-ci.org/arwx/go-sentinel)
[![codecov](https://codecov.io/gh/arwx/go-sentinel/branch/master/graph/badge.svg)](https://codecov.io/gh/arwx/go-sentinel)
[![Go Report Card](https://goreportcard.com/badge/github.com/arwx/go-sentinel)](https://goreportcard.com/report/github.com/arwx/go-sentinel)

Package sentinel implements Redis Sentinel pub/sub watcher for golang

Installation
------------

Install go-sentinel using the "go get" command:

    go get github.com/arwx/go-sentinel

Documentation
-------------
See [godoc](https://godoc.org/github.com/arwx/go-sentinel) for package and API descriptions

Usage
-----

```golang
// create sentinel watcher with minimal config
snt := sentinel.New(sentinel.Config{
	Addrs:             []string{"localhost:26379"},
	Groups:            []string{"example"},
	RefreshInterval:   45 * time.Second,
	HeartbeatInterval: 10 * time.Second,
	HeartbeatTimeout:  5 * time.Second,
})

// run redis instances discovery for configured groups
// and pub/sub listening for +switch-master, +slave, +sdown, -sdown events
go snt.Run()

defer snt.Stop()

// get master address for 'example' master name
master, err := snt.GetMasterAddr("example")

// get slaves addresses for 'example' master name
slaves, err := snt.GetSlavesAddrs("example")
```