package ingestion

import (
	"regexp"
	"strings"
)

type Chunk struct {
	Content, Section string
	Position         int
}

var sentenceBoundary = regexp.MustCompile(`(?s)(.*?[.!?](?:[\"'»)]|\s|$)|.+$)`)
var heading = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)

// ChunkText splits only at sentence boundaries. Overlap is carried forward in whole
// sentences, so a chunk is never cut in the middle of a sentence.
func ChunkText(text string, size, overlap int) []Chunk {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	section := "General"
	var out []Chunk
	var current []string
	currentSection := section
	length := 0
	flush := func() {
		if len(current) == 0 {
			return
		}
		out = append(out, Chunk{Content: strings.TrimSpace(strings.Join(current, " ")), Section: currentSection, Position: len(out)})
		keep := []string{}
		kept := 0
		for i := len(current) - 1; i >= 0; i-- {
			s := strings.TrimSpace(current[i])
			add := len(s)
			if len(keep) > 0 {
				add++
			}
			if kept+add > overlap && len(keep) > 0 {
				break
			}
			keep = append([]string{s}, keep...)
			kept += add
		}
		current, length = keep, kept
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := heading.FindStringSubmatch(line); len(m) == 2 {
			if len(current) > 0 {
				out = append(out, Chunk{Content: strings.TrimSpace(strings.Join(current, " ")), Section: currentSection, Position: len(out)})
				current = nil
				length = 0
			}
			section = strings.TrimSpace(m[1])
			currentSection = section
			continue
		}
		for _, raw := range sentenceBoundary.FindAllString(line, -1) {
			s := strings.TrimSpace(raw)
			if s == "" {
				continue
			}
			add := len(s)
			if len(current) > 0 {
				add++
			}
			if length+add > size && len(current) > 0 {
				flush()
			}
			current = append(current, s)
			length += add
		}
	}
	if len(current) > 0 {
		out = append(out, Chunk{Content: strings.TrimSpace(strings.Join(current, " ")), Section: section, Position: len(out)})
	}
	return out
}
