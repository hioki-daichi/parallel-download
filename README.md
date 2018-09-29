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

## How to try parallel-download using a dummy server

Start a dummy server as folllows,

```shell
$ ./bin/dummy_server.go
=> starting with a failure rate of 0% on http://localhost:8080
================================================================================
THIS IS A DUMMY SERVER THAT CAN PARTIALLY RETURN IMAGE DATA !!
================================================================================
Usage:
  -failure-rate int
        failure rate
  -port int
        port (default 8080)
Endpoint:
  GET /foo.png # Get a gopher image
```

Send a GET /foo.png request from another terminal.

```shell
$ make build
$ ./parallel-download -p 8 http://localhost:8080/foo.png
start HEAD request to get Content-Length
got: Accept-Ranges: bytes
got: Content-Length: 169406
start GET request with header: "Range: bytes=0-21174"
start GET request with header: "Range: bytes=148225-169405"
start GET request with header: "Range: bytes=63525-84699"
start GET request with header: "Range: bytes=21175-42349"
start GET request with header: "Range: bytes=105875-127049"
start GET request with header: "Range: bytes=42350-63524"
start GET request with header: "Range: bytes=84700-105874"
start GET request with header: "Range: bytes=127050-148224"
downloaded: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download876853028/990e6f45-0706-42d0-a794-3ec57f57de30"
downloaded: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download876853028/d2f907b1-d7fc-4d03-906e-0af5ec73b307"
downloaded: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download876853028/a06561a7-f6fd-401f-a639-ab634eafdb53"
downloaded: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download876853028/ed257040-5f58-4122-bcec-b651b726441e"
downloaded: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download876853028/5bd78896-ca15-49be-9d26-8b39dc3789fe"
downloaded: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download876853028/eb2d8908-c887-40f1-a255-bfb4a5c55a37"
downloaded: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download876853028/737c5741-a0c5-476f-96af-50575b4c7cb3"
downloaded: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download876853028/727f4504-cd5d-4b28-b5e4-7ffc967f032c"
create destination tempfile
created: "/var/folders/f8/1n0bk4tj4ll6clyj868k_nqh0000gn/T/parallel-download876853028/28e546a1-39ea-423d-a5fb-0b9c7776d94e"
concat downloaded files to destination tempfile
rename destination tempfile to "foo.png"
completed: "foo.png"
```

## How to run the test

```shell
$ make test
```
