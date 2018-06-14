## go-sentinel

[![Build Status](https://travis-ci.org/ncade/go-sentinel.svg?branch=master)](https://travis-ci.org/ncade/go-sentinel)
[![codecov](https://codecov.io/gh/ncade/go-sentinel/branch/master/graph/badge.svg)](https://codecov.io/gh/ncade/go-sentinel)
[![Go Report Card](https://goreportcard.com/badge/github.com/ncade/go-sentinel)](https://goreportcard.com/report/github.com/ncade/go-sentinel)

Package sentinel implements Redis Sentinel pub/sub watcher for golang

Installation
------------

Install go-sentinel using the "go get" command:

    go get github.com/ncade/go-sentinel

Documentation
-------------
See [godoc](https://godoc.org/github.com/ncade/go-sentinel) for package and API descriptions

Usage
-----

```golang
// create sentinel watcher with minimal config
snt := sentinel.New(sentinel.Config{
	Password:          "password",
	Addrs:             []string{"localhost:26379"},
	Groups:            []string{"redis01", "redis02", "redis03"},
	RefreshInterval:   45 * time.Second,
	HeartbeatInterval: 10 * time.Second,
	HeartbeatTimeout:  5 * time.Second,
})

// run redis instances discovery for configured groups
// and pub/sub listening for +switch-master, +slave, +sdown, -sdown events
go snt.Run()

defer snt.Stop()

// get master address for 'redis01' master name
master, err := snt.GetMasterAddr("redis01")

// get slaves addresses for 'redis01' master name
slaves, err := snt.GetSlavesAddrs("redis01")
```