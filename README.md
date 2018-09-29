# parallel-download

[![Build Status](https://travis-ci.com/hioki-daichi/parallel-download.svg?branch=master)](https://travis-ci.com/hioki-daichi/parallel-download)
[![codecov](https://codecov.io/gh/hioki-daichi/parallel-download/branch/master/graph/badge.svg)](https://codecov.io/gh/hioki-daichi/parallel-download)
[![Go Report Card](https://goreportcard.com/badge/github.com/hioki-daichi/parallel-download)](https://goreportcard.com/report/github.com/hioki-daichi/parallel-download)
[![GoDoc](https://godoc.org/github.com/hioki-daichi/parallel-download?status.svg)](https://godoc.org/github.com/hioki-daichi/parallel-download)

`parallel-download` is a command that can download the resources on the web in parallel.

Available options are below.

| Option | Description                                                                          |
| ---    | ---                                                                                  |
| `-p`   | Download files in parallel according to the specified number. (default 8)            |
| `-o`   | Save the downloaded file in the specified path. (Overwrite if duplicates.)           |
| `-t`   | Terminate when the specified value has elapsed since download started. (default 30s) |

## How to develop

### 1. Install packages

Execute `$ dep ensure` to install dependent packages.

### 2. Start a dummy server

Execute `$ ./bin/dummy_server.go` to start a dummy server that returns a Gopher image.

```
$ ./bin/dummy_server.go
--------------------------------------------------------------------------------
# Endpoint

  GET /foo.png // Get a gopher image

# Command-line options**

  -failure-rate int
        Probability to return InternalServerError.
  -max-delay duration
        Maximum time delay randomly applied from receiving a request until returning a response. (default 1s)
  -port int
        Port on which the dummy server listens. (default 8080)
--------------------------------------------------------------------------------
2018/09/29 16:08:31 Server starting on http://localhost:8080
```

### 3. Execute

Execute the command with specifying the Gopher image endpoint of the dummy server (and some options).

```
$ go run main.go -p=3 -t=10ms -o=bar.png http://localhost:8080/foo.png
start HEAD request to get Content-Length
got: Accept-Ranges: bytes
got: Content-Length: 169406
start GET request with header: "Range: bytes=0-56467"
start GET request with header: "Range: bytes=56468-112935"
start GET request with header: "Range: bytes=112936-169405"
downloaded: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download207726192/998018e7-769d-4ee9-b504-a7895146b791"
downloaded: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download207726192/a59f9202-20d2-46d6-8172-d4df32c5483e"
downloaded: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download207726192/941dd4cf-6a8b-4039-9373-f433f583e2df"
create destination tempfile
created: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download207726192/19c6b4b3-ecc7-45a8-9bff-afd341a46f6a"
concat downloaded files to destination tempfile
rename destination tempfile to "bar.png"
completed: "bar.png"
```

## How to run the test

```shell
$ make test
```
