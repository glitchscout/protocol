// Command compatcheck fails when the schemas in -head aren't backward compatible with -base
// (two checkouts of this repo) without a protocol version bump. CI runs it on every pull request.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/glitchscout/protocol/compat"
)

func main() {
	base := flag.String("base", "", "checkout of the base branch")
	head := flag.String("head", ".", "checkout of the change")
	flag.Parse()
	if *base == "" {
		fmt.Fprintln(os.Stderr, "usage: compatcheck -base DIR [-head DIR]")
		os.Exit(2)
	}
	problems, err := compat.Check(os.DirFS(*base), os.DirFS(*head))
	if err != nil {
		fmt.Fprintln(os.Stderr, "compatcheck:", err)
		os.Exit(2)
	}
	for _, p := range problems {
		fmt.Println("incompatible:", p)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
	fmt.Println("compatible")
}
