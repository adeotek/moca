package calc

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseList parses a comma-separated list of integers ("1,2,3").
func ParseList(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("not a number: %q", p)
		}
		out = append(out, n)
	}
	return out, nil
}
