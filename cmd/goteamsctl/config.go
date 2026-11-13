// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package main

import (
	"fmt"
	"os"
	"strings"
)

// iniEntry is one parsed key=value with its section.
type iniEntry struct {
	Section string
	Key     string
	Value   string
}

// parseIniEntries extracts ordered (section, key, value) triples; comments and blanks are
// skipped. Duplicate keys keep the last value.
func parseIniEntries(content string) []iniEntry {
	var out []iniEntry
	section := ``
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == `` || strings.HasPrefix(line, `;`) || strings.HasPrefix(line, `#`) {
			continue
		}
		if strings.HasPrefix(line, `[`) && strings.HasSuffix(line, `]`) {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		parts := strings.SplitN(line, `=`, 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, iniEntry{
			Section: section,
			Key:     strings.TrimSpace(parts[0]),
			Value:   strings.TrimSpace(parts[1]),
		})
	}
	return out
}

// envEntry is one KEY=VALUE pair in file order.
type envEntry struct {
	Key   string
	Value string
}

// parseEnvOrdered parses KEY=VALUE lines preserving order and skipping comments.
func parseEnvOrdered(content string) []envEntry {
	var out []envEntry
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == `` || strings.HasPrefix(line, `#`) || !strings.Contains(line, `=`) {
			continue
		}
		parts := strings.SplitN(line, `=`, 2)
		out = append(out, envEntry{Key: strings.TrimSpace(parts[0]), Value: strings.TrimSpace(parts[1])})
	}
	return out
}

// mergeEnvIntoOld keeps the old .env verbatim and appends only missing keys, dropping
// unfilled {{placeholder}} values with an explicit error.
func mergeEnvIntoOld(oldContent, newRendered, version string) (string, error) {
	existing := map[string]bool{}
	for _, kv := range parseEnvOrdered(oldContent) {
		existing[kv.Key] = true
	}
	var added []envEntry
	for _, kv := range parseEnvOrdered(newRendered) {
		if existing[kv.Key] {
			continue
		}
		if strings.Contains(kv.Value, `{{`) {
			return ``, fmt.Errorf(`new env key %s has no derivable value (placeholder %s)`, kv.Key, kv.Value)
		}
		added = append(added, kv)
		existing[kv.Key] = true
	}
	if len(added) == 0 {
		return oldContent, nil
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(oldContent, "\n"))
	b.WriteString("\n\n# added by goteamsctl upgrade " + version + "\n")
	for _, kv := range added {
		b.WriteString(kv.Key + `=` + kv.Value + "\n")
	}
	return b.String(), nil
}

// orDefault returns def when value is empty.
func orDefault(value, def string) string {
	if strings.TrimSpace(value) == `` {
		return def
	}
	return value
}

// mustWriteFile wraps writeFileAtomic with a mode and error wrapping.
func mustWriteFile(path string, content string, mode os.FileMode) error {
	return writeFileAtomic(path, []byte(content), mode)
}
