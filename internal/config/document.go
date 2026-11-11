// Package config reads and writes Code's settings, a plain text file in
// Ghostty's syntax, and derives the values the terminals use from it.
package config

import (
	"strings"
)

// Document is a config file in Ghostty's syntax, kept line by line so a setting
// can be changed without touching the comments or the order around it.
type Document struct {
	lines []string
}

// ParseDocument splits text into lines, dropping a trailing empty line.
func ParseDocument(text string) *Document {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return &Document{lines: lines}
}

// Text renders the document with a trailing newline per line.
func (d *Document) Text() string {
	var b strings.Builder
	for _, line := range d.lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// IsEmpty reports whether no line sets anything.
func (d *Document) IsEmpty() bool {
	for _, line := range d.lines {
		if _, _, ok := entry(line); ok {
			return false
		}
	}
	return true
}

// Values returns every value set for key, in order. Ghostty uses the last one.
func (d *Document) Values(key string) []string {
	var out []string
	for _, line := range d.lines {
		if k, v, ok := entry(line); ok && k == key {
			out = append(out, v)
		}
	}
	return out
}

// Get returns the last value of a key, and whether any line set it.
func (d *Document) Get(key string) (string, bool) {
	values := d.Values(key)
	if len(values) == 0 {
		return "", false
	}
	return values[len(values)-1], true
}

// Keys returns every key the document sets, once, in the order it first sets
// them.
func (d *Document) Keys() []string {
	seen := make(map[string]bool, len(d.lines))
	var out []string
	for _, line := range d.lines {
		if key, _, ok := entry(line); ok && !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

// SetValues replaces the lines setting key with one line per value, at the
// position of the first of them, or at the end. No values removes the key.
func (d *Document) SetValues(values []string, key string) {
	indices := make([]int, 0, len(d.lines))
	for i, line := range d.lines {
		if k, _, ok := entry(line); ok && k == key {
			indices = append(indices, i)
		}
	}
	position := len(d.lines)
	if len(indices) > 0 {
		position = indices[0]
	}
	kept := make([]string, 0, len(d.lines)+len(values))
	kept = append(kept, d.lines[:position]...)
	for _, v := range values {
		kept = append(kept, key+" = "+formatValue(v))
	}
	// Only a key that was already set has lines after it to keep; a new key
	// is appended, so nothing follows it.
	if len(indices) > 0 {
		kept = append(kept, d.lines[indices[len(indices)-1]+1:]...)
	}
	d.lines = kept
}

// Set replaces a key with a single value. An empty value removes the key, so
// Ghostty's default applies.
func (d *Document) Set(key, value string) {
	if value == "" {
		d.Remove(key)
		return
	}
	d.SetValues([]string{value}, key)
}

// Remove drops every line setting key.
func (d *Document) Remove(key string) {
	d.SetValues(nil, key)
}

// entry reads a line that sets something the way Ghostty reads it: comments
// are whole lines, a carriage return is whitespace, quotes around a value are
// dropped, and a key on its own turns it on.
func entry(line string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	equals := strings.IndexByte(trimmed, '=')
	if equals < 0 {
		return trimmed, "true", true
	}
	key = strings.TrimSpace(trimmed[:equals])
	value = strings.TrimSpace(trimmed[equals+1:])
	if len(value) >= 2 && strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		value = value[1 : len(value)-1]
	}
	return key, value, true
}

func formatValue(value string) string {
	if value == "" || value != strings.TrimSpace(value) {
		return "\"" + value + "\""
	}
	return value
}
