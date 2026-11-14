// Command tasksvc is a tiny task queue backed by one JSON file.
package main

import (
	"fmt"
	"os"

	"example.com/tasksvc/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: tasksvc <add|list|archive|index|export> [args]")
		os.Exit(2)
	}
	st := store.New("tasks.json")
	var err error
	switch os.Args[1] {
	case "add":
		title := "untitled"
		if len(os.Args) > 2 {
			title = os.Args[2]
		}
		err = st.Save(store.Task{ID: title, Title: title})
	case "list":
		var all []store.Task
		all, err = st.Load()
		for _, t := range all {
			fmt.Printf("%-12s %-30s done=%v\n", t.ID, t.Title, t.Done)
		}
	case "index":
		var idx map[string]store.Task
		idx, err = st.Index()
		fmt.Printf("%d tasks indexed\n", len(idx))
	case "archive":
		var all []store.Task
		all, err = st.Load()
		var done []store.Task
		for _, t := range all {
			if t.Done {
				done = append(done, t)
			}
		}
		err = st.Archive(done)
	case "export":
		var all []store.Task
		all, err = st.Load()
		dir := "export"
		_ = os.MkdirAll(dir, 0o755)
		err = store.Export(dir, all)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
