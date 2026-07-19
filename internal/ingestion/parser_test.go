package ingestion

import (
	"strings"
	"testing"
)

func TestExtractHTMLDropsExecutableContent(t *testing.T) {
	got, err := ExtractText("html", "<h1>Fees</h1><script>ignore me</script><style>x{}</style><p>EUR 190.</p>")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "ignore") || !strings.Contains(got, "EUR 190") {
		t.Fatalf("unexpected text: %q", got)
	}
}
