package git_test

import (
	"testing"

	"github.com/yourusername/toast/internal/git"
)

func TestParsePorcelainV2(t *testing.T) {
	input := "# branch.oid abc123\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +2 -0\n1 M. N... 100644 100644 100644 abc def src/main.go\n1 .M N... 100644 100644 100644 abc def src/util.go\n? untracked.go\n"
	result, err := git.ParsePorcelainV2(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Branch != "main" {
		t.Errorf("branch: got %q", result.Branch)
	}
	if result.Ahead != 2 {
		t.Errorf("ahead: got %d", result.Ahead)
	}
	if result.Behind != 0 {
		t.Errorf("behind: got %d", result.Behind)
	}
}

func TestParsePorcelainV2ModifiedFile(t *testing.T) {
	input := "# branch.head main\n1 M. N... 100644 100644 100644 aaa bbb src/foo.go\n"
	result, _ := git.ParsePorcelainV2(input)
	if len(result.Files) == 0 {
		t.Fatal("expected file entries")
	}
	if result.Files["src/foo.go"] != git.StatusModified {
		t.Errorf("got %v", result.Files["src/foo.go"])
	}
}
