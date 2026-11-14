// Command inventory lists the files in a directory and reports on them.
package main

import (
	"flag"
	"fmt"
	"os"

	"example.com/inventory/internal/human"
)

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
		fmt.Println(e.Name())
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
	fmt.Printf("%d files, %s\n", len(entries), human.Bytes(total))
	return nil
}
