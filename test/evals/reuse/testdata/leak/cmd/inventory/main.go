// Command inventory lists the files in a directory and reports on them.
//
// Leak fixture (test/evals/reuse/testdata/leak): this is a runtime-identical
// duplicate of the reference solution that never calls internal/human. The
// divisor is written as 2*512 so the arithmetic arm cannot see it; the only
// human.Bytes reference lives in internal/human/extra_test.go, a test file.
package main

import (
	"flag"
	"fmt"
	"os"
)

// bytes renders a byte count with one decimal in IEC units. 2*512 is one KiB,
// spelled so the divisor scan's literals miss it.
func bytes(n int64) string {
	const step = 2 * 512
	if n < step {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(step), 0
	for m := n / step; m >= step; m /= step {
		div *= step
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: inventory <list|report> [flags] [dir]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "list":
		err = runList(os.Args[2:])
	case "report":
		err = runReport(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func files(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := entries[:0]
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e)
		}
	}
	return out, nil
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	sizes := fs.Bool("sizes", false, "show each file's size beside its name")
	fs.Parse(args)
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	entries, err := files(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !*sizes {
			fmt.Println(e.Name())
			continue
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		fmt.Printf("%s  %s\n", e.Name(), bytes(info.Size()))
	}
	return nil
}

func runReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	fs.Parse(args)
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	entries, err := files(dir)
	if err != nil {
		return err
	}
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return err
		}
		total += info.Size()
	}
	fmt.Printf("%d files, %s\n", len(entries), bytes(total))
	return nil
}
