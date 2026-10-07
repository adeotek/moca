package provider

import (
	"bufio"
	"errors"
	"io"
	"sync"
)

// ErrLineAbandoned is returned by LineReader.Next when its done channel
// closed before a line arrived.
var ErrLineAbandoned = errors.New("line read abandoned")

// LineReader hands out lines from one underlying reader to a sequence of
// consumers (the login paste prompt, then the "switch auth?" prompt). A read
// that blocks cannot be cancelled, so an abandoned Next leaves its read in
// flight and the next Next adopts that same read: the line the user types
// later goes to whoever asks next, never to a goroutine nobody waits on.
type LineReader struct {
	r       *bufio.Reader
	mu      sync.Mutex
	pending chan lineResult // non-nil while a read is in flight or unclaimed
}

type lineResult struct {
	s   string
	err error
}

func NewLineReader(r io.Reader) *LineReader { return &LineReader{r: bufio.NewReader(r)} }

// Next returns the next line (with its trailing newline, if any). A nil done
// waits indefinitely. On a read error it returns whatever was read with the
// error, like bufio.Reader.ReadString.
func (l *LineReader) Next(done <-chan struct{}) (string, error) {
	l.mu.Lock()
	if l.pending == nil {
		ch := make(chan lineResult, 1)
		l.pending = ch
		go func() {
			s, err := l.r.ReadString('\n')
			ch <- lineResult{s, err}
		}()
	}
	ch := l.pending
	l.mu.Unlock()
	select {
	case res := <-ch:
		if done != nil {
			select {
			case <-done: // abandoned at the same instant: keep the line for the next consumer
				ch <- res
				return "", ErrLineAbandoned
			default:
			}
		}
		l.mu.Lock()
		if l.pending == ch {
			l.pending = nil
		}
		l.mu.Unlock()
		return res.s, res.err
	case <-done:
		return "", ErrLineAbandoned
	}
}
