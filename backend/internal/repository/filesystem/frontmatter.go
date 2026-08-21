package filesystem

import (
	"errors"
	"strings"
)

var errNoFrontMatter = errors.New("missing or malformed YAML front matter delimiters")

// splitFrontMatter separates a canonical node file into its YAML front matter and its
// markdown body.
//
// The delimiter contract matches tools/validate-graph.ps1: the file must open with a "---"
// line and the front matter must be terminated by the next "---" line. Both LF and CRLF
// line endings are accepted so a clone made with Git's Windows defaults still parses.
func splitFrontMatter(raw string) (frontMatter string, body string, err error) {
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	text = strings.TrimPrefix(text, "\ufeff")

	if !strings.HasPrefix(text, "---\n") {
		return "", "", errNoFrontMatter
	}
	rest := text[len("---\n"):]

	end := strings.Index(rest, "\n---")
	for end >= 0 {
		after := rest[end+len("\n---"):]
		if after == "" || strings.HasPrefix(after, "\n") {
			return rest[:end], strings.TrimPrefix(strings.TrimPrefix(after, "\n"), "\n"), nil
		}
		next := strings.Index(rest[end+1:], "\n---")
		if next < 0 {
			break
		}
		end = end + 1 + next
	}
	return "", "", errNoFrontMatter
}
