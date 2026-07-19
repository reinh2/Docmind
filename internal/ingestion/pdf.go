package ingestion

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// extractPDF delegates the format-specific parsing to Poppler's pdftotext. The
// runtime image supplies it; execution has no shell, a hard timeout, and no path
// controlled by the upload.
func extractPDF(body string) (string, error) {
	if !strings.HasPrefix(body, "%PDF-") {
		return "", fmt.Errorf("invalid PDF header")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pdftotext", "-", "-")
	cmd.Stdin = strings.NewReader(body)
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return "", fmt.Errorf("PDF extraction timed out")
	}
	if err != nil {
		return "", fmt.Errorf("PDF extraction failed: %w", err)
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return "", fmt.Errorf("PDF contains no extractable text")
	}
	return text, nil
}
