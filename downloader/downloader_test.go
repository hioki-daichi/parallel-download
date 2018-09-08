package downloader

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestDownloader_Download_IsFileExist(t *testing.T) {
	t.Parallel()

	_, err := os.Create("foo.txt")
	if err != nil {
		t.Fatalf("err %s", err)
	}
	defer os.Remove("foo.txt")

	d := NewDownloader(ioutil.Discard, "https://example.com/foo.txt")
	actual := d.Download()
	expected := errExist
	if actual != expected {
		t.Errorf(`expected="%s" actual="%s"`, expected, actual)
	}
}
