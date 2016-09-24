# di

[![GoDoc](https://godoc.org/github.com/kkrs/di?status.svg)](https://godoc.org/github.com/kkrs/di)
[![Travis CI](https://travis-ci.org/kkrs/di.svg?branch=master)](https://travis-ci.org/kkrs/di) 
[![Coverage Status](https://coveralls.io/repos/github/kkrs/di/badge.svg?branch=master)](https://coveralls.io/github/kkrs/di?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/kkrs/di)](https://goreportcard.com/report/github.com/kkrs/di)

Package di is a go Dependency Injection library for web development. [
Dependency Injection and Testable Web Development in Go](http://blog.extremix.net/post/di/)
provides rationale for Dependency Injection for web development and its application using di.

## Install
With a [correctly configured](https://golang.org/doc/install#testing) Go toolchain:

```
go get -d github.com/kkrs/di
go get -d github.com/kkrs/di/router
```

## Status
This package is experimental and may change.

## Inspiration
di is roughly modeled after the design in [Where Have All the Singletons Gone?](
http://misko.hevery.com/2008/08/21/where-have-all-the-singletons-gone/).

## Licence
MIT
