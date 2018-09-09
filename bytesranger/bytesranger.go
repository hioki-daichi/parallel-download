package bytesranger

import (
	"errors"
	"fmt"
)

var (
	errZeroDivision       = errors.New("bytesranger: zero division error")
	errNumberOfPartitions = errors.New("bytesranger: numberOfPartitions must be less than or equal to number")
)

// Split splits the number into the specified numberOfPartitions and changes to bytes range strings.
func Split(number int, numberOfPartitions int) ([]string, error) {
	err := validate(number, numberOfPartitions)
	if err != nil {
		return nil, err
	}

	return toBytesRangeStrings(toBytesRanges(number, numberOfPartitions)), nil
}

type bytesRange struct {
	first int
	last  int
}

func validate(number int, numberOfPartitions int) error {
	if numberOfPartitions == 0 {
		return errZeroDivision
	}

	if number < numberOfPartitions {
		return errNumberOfPartitions
	}

	return nil
}

func toBytesRanges(number int, numberOfPartitions int) []bytesRange {
	ret := make([]bytesRange, 0)

	numberPerUnit := number / numberOfPartitions

	n := 0

	for i := numberOfPartitions; i > 0; i-- {
		br := bytesRange{first: n, last: n + numberPerUnit - 1}
		ret = append(ret, br)
		n += numberPerUnit
	}

	if rem := number % numberOfPartitions; rem != 0 {
		ret[len(ret)-1].last += rem
	}

	return ret
}

func toBytesRangeStrings(bytesRanges []bytesRange) []string {
	ret := make([]string, 0)

	for _, br := range bytesRanges {
		ret = append(ret, fmt.Sprintf("bytes=%d-%d", br.first, br.last))
	}

	return ret
}
