package provider

import (
	"bufio"
	"context"
	"io"
	"strings"
	"time"
)

// readSSE parses a text/event-stream. fn receives each dispatched event's
// type ("" when absent) and data (multi-line data joined with "\n"). If no
// line arrives within stall, it returns ErrStall; the caller cancels the
// request so the body read unblocks.
func readSSE(ctx context.Context, body io.Reader, stall time.Duration, fn func(event, data string) error) error {
	lines := make(chan string)
	errc := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(body)
		sc.Buffer(make([]byte, 64<<10), 8<<20)
		for sc.Scan() {
			select {
			case lines <- strings.TrimSuffix(sc.Text(), "\r"):
			case <-ctx.Done():
				return
			}
		}
		errc <- sc.Err()
	}()

	var event string
	var data []string
	dispatch := func() error {
		if len(data) == 0 {
			event = ""
			return nil
		}
		err := fn(event, strings.Join(data, "\n"))
		event, data = "", nil
		return err
	}
	timer := time.NewTimer(stall)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return ErrStall
		case err := <-errc:
			if err != nil {
				return err
			}
			return dispatch()
		case line := <-lines:
			timer.Reset(stall)
			switch {
			case line == "":
				if err := dispatch(); err != nil {
					return err
				}
			case strings.HasPrefix(line, ":"):
			default:
				field, value, _ := strings.Cut(line, ":")
				value = strings.TrimPrefix(value, " ")
				switch field {
				case "event":
					event = value
				case "data":
					data = append(data, value)
				}
			}
		}
	}
}
