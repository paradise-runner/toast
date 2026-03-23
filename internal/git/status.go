package git

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type FileStatus int

const (
	StatusClean FileStatus = iota
	StatusModified
	StatusAdded
	StatusDeleted
	StatusUntracked
	StatusConflict
)

type StatusResult struct {
	Branch        string
	Ahead, Behind int
	Files         map[string]FileStatus
}

func Run(rootDir string) (*StatusResult, error) {
	cmd := exec.Command("git", "status", "--porcelain=v2", "--branch")
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	return ParsePorcelainV2(string(out))
}

func ParsePorcelainV2(output string) (*StatusResult, error) {
	result := &StatusResult{Files: make(map[string]FileStatus)}
	for _, line := range strings.Split(output, "\n") {
		if len(line) == 0 {
			continue
		}
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			result.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.ab "):
			parts := strings.Fields(strings.TrimPrefix(line, "# branch.ab "))
			if len(parts) == 2 {
				if n, err := strconv.Atoi(strings.TrimPrefix(parts[0], "+")); err == nil {
					result.Ahead = n
				}
				if n, err := strconv.Atoi(strings.TrimPrefix(parts[1], "-")); err == nil {
					result.Behind = n
				}
			}
		case strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 "):
			fields := strings.Fields(line)
			if len(fields) < 9 {
				continue
			}
			result.Files[fields[8]] = xyToStatus(fields[1])
		case strings.HasPrefix(line, "? "):
			result.Files[strings.TrimSuffix(strings.TrimPrefix(line, "? "), "/")] = StatusUntracked
		case strings.HasPrefix(line, "u "):
			fields := strings.Fields(line)
			if len(fields) >= 9 {
				result.Files[fields[8]] = StatusConflict
			}
		}
	}
	return result, nil
}

func xyToStatus(xy string) FileStatus {
	if len(xy) < 2 {
		return StatusClean
	}
	x, y := rune(xy[0]), rune(xy[1])
	if x == '!' || y == '!' {
		return StatusConflict
	}
	if x == 'A' || y == 'A' {
		return StatusAdded
	}
	if x == 'D' || y == 'D' {
		return StatusDeleted
	}
	if x == 'M' || y == 'M' {
		return StatusModified
	}
	return StatusClean
}
