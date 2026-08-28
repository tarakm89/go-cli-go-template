package cli

import "strings"

// joinWords renders a list of allowed values for flag help.
func joinWords(values []string) string { return strings.Join(values, ", ") }
