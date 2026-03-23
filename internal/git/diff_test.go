package git_test

import (
	"testing"

	"github.com/yourusername/toast/internal/git"
	"github.com/yourusername/toast/internal/messages"
)

func TestParseDiff(t *testing.T) {
	input := "--- a/main.go\n+++ b/main.go\n@@ -1,6 +1,6 @@\n line1\n line2\n+line3new\n line4\n-line5\n line6\n"
	kinds := git.ParseDiff(input, 6)
	if kinds[2] != messages.GitLineAdded {
		t.Errorf("line 2: expected Added, got %v", kinds[2])
	}
	if kinds[4] != messages.GitLineDeleted {
		t.Errorf("line 4: expected Deleted, got %v", kinds[4])
	}
}
