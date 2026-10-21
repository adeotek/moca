package config

import _ "embed"

// sampleText is the documented, complete user-level config printed by
// `moca config sample` (SPECS §3). Every option appears, either active at its
// built-in default or commented with its default/example; options that REPLACE
// a default (shell.allow) or are environment-specific stay commented, so
// copying the file unchanged is a no-op. TestSampleParses keeps it valid and
// TestSampleCoversEveryKey keeps it complete as the schema grows.
//
//go:embed sample.jsonc
var sampleText string

// Sample returns the user-level config template. The caller prints it; the
// returned text is the embedded file byte for byte.
func Sample() string { return sampleText }
