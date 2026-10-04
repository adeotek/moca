package tools

// ReadTracker remembers the on-disk state of every file read (or written) in
// this session. Full implementation lands in task 3; tool.go's Env needs the
// type to exist from task 1 on.
type ReadTracker struct{}
