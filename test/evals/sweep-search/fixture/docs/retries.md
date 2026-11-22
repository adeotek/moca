# Client retries

The HTTP client retries a failed request 5 times. The first retry waits 400 ms
and every subsequent wait doubles, capped at 4 s. Only idempotent requests are
retried; a 4xx is never retried.
