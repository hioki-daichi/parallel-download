package bytesranger

import (
	"reflect"
	"testing"
)

func TestBytesranger_Split(t *testing.T) {
	cases := map[string]struct {
		number             int
		numberOfPartitions int
		expected           []string
	}{
		"split 5 into 1": {number: 5, numberOfPartitions: 1, expected: []string{"bytes=0-4"}},
		"split 5 into 2": {number: 5, numberOfPartitions: 2, expected: []string{"bytes=0-1", "bytes=2-4"}},
		"split 5 into 3": {number: 5, numberOfPartitions: 3, expected: []string{"bytes=0-0", "bytes=1-1", "bytes=2-4"}},
		"split 5 into 4": {number: 5, numberOfPartitions: 4, expected: []string{"bytes=0-0", "bytes=1-1", "bytes=2-2", "bytes=3-4"}},
		"split 5 into 5": {number: 5, numberOfPartitions: 5, expected: []string{"bytes=0-0", "bytes=1-1", "bytes=2-2", "bytes=3-3", "bytes=4-4"}},
	}

	for n, c := range cases {
		c := c
		t.Run(n, func(t *testing.T) {
			t.Parallel()

			number := c.number
			numberOfPartitions := c.numberOfPartitions
			expected := c.expected

			actual, err := Split(number, numberOfPartitions)
			if err != nil {
				t.Fatalf("err %s", err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf(`expected="%s" actual="%s"`, expected, actual)
			}
		})
	}
}

func TestBytesranger_Split_ZeroDivisionError(t *testing.T) {
	actual, err := Split(10, 0)
	if len(actual) != 0 {
		t.Fatalf("expected that length is 0, but %d", len(actual))
	}
	if err != errZeroDivision {
		t.Errorf(`expected="%v" actual="%v"`, errZeroDivision, err)
	}
}

func TestBytesranger_Split_NumberOfPartitionsError(t *testing.T) {
	actual, err := Split(10, 11)
	if len(actual) != 0 {
		t.Fatalf("expected that length is 0, but %d", len(actual))
	}
	if err != errNumberOfPartitions {
		t.Errorf(`expected="%v" actual="%v"`, errNumberOfPartitions, err)
	}
}
