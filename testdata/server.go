package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var contents string

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	opts := parse()
	setContents(opts.path)
	http.HandleFunc("/foo.png", handler)
	http.ListenAndServe(opts.addr, nil)
}

func handler(w http.ResponseWriter, req *http.Request) {
	time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)

	w.Header().Set("Accept-Ranges", "bytes")
	body, err := genBody(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, err.Error())
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	fmt.Fprint(w, body)
}

func genBody(req *http.Request) (string, error) {
	// e.g. "bytes=0-99"
	rangeHdr := req.Header.Get("Range")
	if rangeHdr == "" {
		return contents, nil
	}

	// e.g. []string{"bytes", "0-99"}
	eqlSplitVals := strings.Split(rangeHdr, "=")
	if eqlSplitVals[0] != "bytes" {
		return "", errors.New(`only "bytes" is accepted`)
	}

	// e.g. []string{"0", "99"}
	c := strings.Split(eqlSplitVals[1], "-")

	// e.g. 0
	start, err := strconv.Atoi(c[0])
	if err != nil {
		return "", err
	}

	// e.g. 99
	end, err := strconv.Atoi(c[1])
	if err != nil {
		return "", err
	}

	// e.g. "Range: bytes=1-0"
	if start > end {
		return "", errors.New("invalid range")
	}

	l := len(contents)
	if end > l {
		end = l
	}

	return contents[start : end+1], nil
}

func parse() *options {
	flg := flag.NewFlagSet("test", flag.ExitOnError)
	port := flg.Int("port", 8080, "port")
	path := flg.String("f", "./testdata/foo.png", "path")
	flg.Parse(os.Args[1:])
	addr := ":" + strconv.Itoa(*port)
	return &options{addr: addr, path: *path}
}

type options struct {
	addr string
	path string
}

func setContents(path string) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	contents = string(b)
}
