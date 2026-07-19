package ingestion

import (
	"fmt"
	"regexp"
	"strings"
)

var scripts = regexp.MustCompile(`(?is)<script[^>]*>.*?</script\s*>`)
var styles = regexp.MustCompile(`(?is)<style[^>]*>.*?</style\s*>`)
var tags = regexp.MustCompile(`(?s)<[^>]+>`)

func ExtractText(sourceType, body string) (string, error) {
	switch strings.ToLower(sourceType) {
	case "txt", "text", "md", "markdown":
		return strings.TrimSpace(body), nil
	case "html", "htm":
		clean := scripts.ReplaceAllString(body, " ")
		clean = styles.ReplaceAllString(clean, " ")
		clean = tags.ReplaceAllString(clean, " ")
		return strings.TrimSpace(strings.Join(strings.Fields(clean), " ")), nil
	case "pdf":
		return extractPDF(body)
	default:
		return "", fmt.Errorf("unsupported source type %q", sourceType)
	}
}
