// Command inventory prints the stock report.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	asJSON := flag.Bool("json", false, "print the items as JSON")
	flag.Parse()
	if *asJSON {
		s, err := RenderJSON(Items)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(s)
		return
	}
	fmt.Print(Render(Items))
}
