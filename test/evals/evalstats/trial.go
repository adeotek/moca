package main

// Trial transcript checks (codemode plan v13, Task 1 / E2). One two-turn
// trial is two processes over one session: the first answers prompt one, the
// second resumes it with --continue and answers the follow-up. The harness has
// to prove the second process RESUMED — a fresh session would look like a pass
// with a cheaper history for the wrong reason — and that the two prompts
// landed in their own turns. Both are transcript facts, so they are checked
// here, in Go, rather than by grepping JSONL in bash.

import (
	"fmt"
	"strings"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// CheckTrialSession verifies a schema-2 transcript: it opens with the session
// header, the first turn's prompt is the first user text, an assistant answer
// follows it, the follow-up is the only later user text, and a second answer
// follows that. Every failure names the turn it found instead.
func CheckTrialSession(path, prompt1, prompt2 string) error {
	entries, err := session.ReadFile(path)
	if err != nil {
		return err
	}
	if len(entries) == 0 || entries[0].Type != session.TypeSession {
		return fmt.Errorf("transcript does not open with a session header")
	}
	want := []string{strings.TrimSpace(prompt1), strings.TrimSpace(prompt2)}
	var users []int // indices of user text messages, in order
	for i, e := range entries {
		if e.Type != session.TypeMessage || e.Message == nil || e.Message.Role != llm.RoleUser {
			continue
		}
		if strings.TrimSpace(llm.TextOf(*e.Message)) == "" {
			continue // a nudge-only or tool-result user message is not a prompt
		}
		users = append(users, i)
	}
	if len(users) != 2 {
		return fmt.Errorf("found %d user prompts, want 2 (a new session instead of --continue, or a merged turn)", len(users))
	}
	for k, i := range users {
		got := strings.TrimSpace(llm.TextOf(*entries[i].Message))
		if got != want[k] {
			return fmt.Errorf("turn %d prompt = %q, want %q (wrong turn boundary)", k+1, truncate(got), truncate(want[k]))
		}
	}
	answerBetween := func(from, to int) bool {
		for i := from + 1; i < to; i++ {
			e := entries[i]
			if e.Type == session.TypeMessage && e.Message != nil && e.Message.Role == llm.RoleAssistant &&
				strings.TrimSpace(llm.TextOf(*e.Message)) != "" {
				return true
			}
		}
		return false
	}
	if !answerBetween(users[0], users[1]) {
		return fmt.Errorf("turn 1 has no answer before the follow-up (missing first answer)")
	}
	if !answerBetween(users[1], len(entries)) {
		return fmt.Errorf("turn 2 has no answer")
	}
	return nil
}

func truncate(s string) string {
	if len(s) <= 60 {
		return s
	}
	return s[:60] + "…"
}
