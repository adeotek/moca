// Command inventory prints the stock report.
package main

import (
	"flag"
	"fmt"
)

func main() {
	flag.Parse()
	fmt.Print(Render(Items))
}
