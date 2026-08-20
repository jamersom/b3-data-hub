package b3

import (
	"os"
	"strings"
	"testing"
)

func TestDownloadToTemporaryFile(t *testing.T) {
	t.Run("reads content within the limit", func(t *testing.T) {
		result, err := downloadToTemporaryFile(strings.NewReader("12345"), 5)
		if err != nil {
			t.Fatalf("downloadToTemporaryFile() error = %v", err)
		}
		defer os.Remove(result.path)
		if result.size != 5 {
			t.Fatalf("size = %d, want 5", result.size)
		}
	})

	t.Run("rejects content above the limit", func(t *testing.T) {
		result, err := downloadToTemporaryFile(strings.NewReader("123456"), 5)
		if err == nil {
			t.Fatal("readWithLimit() error = nil, want size limit error")
		}
		if result.path != "" {
			t.Fatalf("path = %q, want empty", result.path)
		}
		if !strings.Contains(err.Error(), "exceeds the 5-byte limit") {
			t.Fatalf("readWithLimit() error = %q, want size limit message", err)
		}
	})
}
