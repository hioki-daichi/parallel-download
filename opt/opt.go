/*
Package opt deals with CLI options.
*/
package opt

import "flag"

// Options has the options required for parallel-download.
type Options struct {
	Parallelism int
	Output      string
}

// Parse parses args and returns Options and url.
func Parse(args ...string) (*Options, string) {
	flg := flag.NewFlagSet("parallel-download", flag.ExitOnError)

	parallelism := flg.Int("p", 8, "parallelism")
	output := flg.String("o", "", "output")

	flg.Parse(args)

	url := flg.Arg(0)

	return &Options{
		Parallelism: *parallelism,
		Output:      *output,
	}, url
}
