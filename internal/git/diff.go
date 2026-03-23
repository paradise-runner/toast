package git

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/yourusername/toast/internal/messages"
)

func RunDiff(rootDir, path string, lineCount int) ([]messages.GitLineKind, error) {
	cmd := exec.Command("git", "diff", "HEAD", "--", path)
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		return make([]messages.GitLineKind, lineCount), nil
	}
	return ParseDiff(string(out), lineCount), nil
}

func ParseDiff(diff string, lineCount int) []messages.GitLineKind {
	kinds := make([]messages.GitLineKind, lineCount)
	var newLine int
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "@@ ") {
			parts := strings.Fields(line)
			if len(parts) < 3 {
				continue
			}
			newPart := strings.TrimPrefix(parts[2], "+")
			newPart = strings.SplitN(newPart, ",", 2)[0]
			if n, err := strconv.Atoi(newPart); err == nil {
				newLine = n - 1
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			if newLine >= 0 && newLine < lineCount {
				kinds[newLine] = messages.GitLineAdded
			}
			newLine++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			if newLine >= 0 && newLine < lineCount {
				if kinds[newLine] == messages.GitLineUnchanged {
					kinds[newLine] = messages.GitLineDeleted
				}
			}
		case strings.HasPrefix(line, " "):
			newLine++
		}
	}
	return kinds
}

func RunDiffForBuffer(rootDir, path string, bufferID, lineCount int) ([]messages.GitLineKind, error) {
	kinds, err := RunDiff(rootDir, path, lineCount)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	return kinds, nil
}
