package main

import (
	"io/ioutil"
	"testing"
)

func TestMain_execute(t *testing.T) {
	url := "https://cdn.kernel.org/pub/linux/kernel/v4.x/linux-4.18.6.tar.xz"
	execute(ioutil.Discard, url)
}
