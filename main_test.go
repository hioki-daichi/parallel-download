package main

import (
	"reflect"
	"testing"
)

func TestMain_toRangeStrings(t *testing.T) {
	cases := map[string]struct {
		contentLength int
		parallelism   int
		expected      []string
	}{
		"contentLength: 5, parallelism: 0": {contentLength: 5, parallelism: 0, expected: []string{"bytes=0-4"}},
		"contentLength: 5, parallelism: 1": {contentLength: 5, parallelism: 1, expected: []string{"bytes=0-4"}},
		"contentLength: 5, parallelism: 2": {contentLength: 5, parallelism: 2, expected: []string{"bytes=0-1", "bytes=2-4"}},
		"contentLength: 5, parallelism: 3": {contentLength: 5, parallelism: 3, expected: []string{"bytes=0-0", "bytes=1-1", "bytes=2-4"}},
		"contentLength: 5, parallelism: 4": {contentLength: 5, parallelism: 4, expected: []string{"bytes=0-0", "bytes=1-1", "bytes=2-2", "bytes=3-4"}},
		"contentLength: 5, parallelism: 5": {contentLength: 5, parallelism: 5, expected: []string{"bytes=0-0", "bytes=1-1", "bytes=2-2", "bytes=3-3", "bytes=4-4"}},
		"contentLength: 5, parallelism: 6": {contentLength: 5, parallelism: 6, expected: []string{"bytes=0-0", "bytes=1-1", "bytes=2-2", "bytes=3-3", "bytes=4-4"}},
	}

	for n, c := range cases {
		c := c
		t.Run(n, func(t *testing.T) {
			t.Parallel()

			contentLength := c.contentLength
			parallelism := c.parallelism
			expected := c.expected

			actual, err := toRangeStrings(contentLength, parallelism)
			if err != nil {
				t.Fatalf("err %s", err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf(`expected="%s" actual="%s"`, expected, actual)
			}
		})
	}
}
