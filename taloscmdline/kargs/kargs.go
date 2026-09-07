// Package kargs merges kernel command line arguments.
package kargs

import (
	"slices"
	"strings"
)

// Merge applies the arguments in set to the existing command line. A
// key=value argument replaces every existing argument with the same key,
// keeping the position of the first one; a bare flag is appended once.
// Whitespace is normalised to single spaces.
func Merge(existing, set string) string {
	current := strings.Fields(existing)

	for _, arg := range strings.Fields(set) {
		key, _, hasValue := strings.Cut(arg, "=")
		if !hasValue {
			if !slices.Contains(current, arg) {
				current = append(current, arg)
			}

			continue
		}

		replaced := false
		kept := current[:0]

		for _, c := range current {
			if cKey, _, cHasValue := strings.Cut(c, "="); cHasValue && cKey == key {
				if !replaced {
					kept = append(kept, arg)
					replaced = true
				}

				continue
			}

			kept = append(kept, c)
		}

		current = kept

		if !replaced {
			current = append(current, arg)
		}
	}

	return strings.Join(current, " ")
}
