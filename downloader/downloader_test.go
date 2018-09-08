package downloader

import (
	"bytes"
	"testing"
)

func TestDownloader_Download(t *testing.T) {
	t.Parallel()

	url := "https://cdn.kernel.org/pub/linux/kernel/v4.x/linux-4.18.6.tar.xz"
	expected := "Downloaded: \"" + url + "\"\n"

	var buf bytes.Buffer
	d := NewDownloader(&buf, url)
	d.Download()

	actual := buf.String()
	if actual != expected {
		t.Errorf(`expected="%s" actual="%s"`, expected, actual)
	}
}
