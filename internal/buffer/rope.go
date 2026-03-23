// Package buffer provides the core text buffer implementation using a rope data structure.
// A rope is a balanced binary tree that enables O(log n) insert, delete, and line lookups.
package buffer

import "strings"

const leafMaxSize = 512

// ropeNode is a node in the rope tree.
// Leaf nodes have text set and left/right nil.
// Internal nodes have left and/or right set and text empty.
type ropeNode struct {
	text      string
	left      *ropeNode
	right     *ropeNode
	length    int // total byte length of subtree
	lineCount int // number of complete lines (newlines) in subtree
}

// newLeaf creates a leaf node from the given string.
func newLeaf(s string) *ropeNode {
	return &ropeNode{
		text:      s,
		length:    len(s),
		lineCount: strings.Count(s, "\n"),
	}
}

// isLeaf reports whether this node is a leaf node.
func (n *ropeNode) isLeaf() bool {
	return n.left == nil && n.right == nil
}

// concat creates an internal node joining left and right subtrees.
// Either child may be nil; if both are nil, returns nil.
func concat(left, right *ropeNode) *ropeNode {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	return &ropeNode{
		left:      left,
		right:     right,
		length:    left.length + right.length,
		lineCount: left.lineCount + right.lineCount,
	}
}

// collect appends all leaf text to buf (in-order traversal).
func collect(n *ropeNode, buf *strings.Builder) {
	if n == nil {
		return
	}
	if n.isLeaf() {
		buf.WriteString(n.text)
		return
	}
	collect(n.left, buf)
	collect(n.right, buf)
}

// nodeString returns the full text of a subtree.
func nodeString(n *ropeNode) string {
	if n == nil {
		return ""
	}
	if n.isLeaf() {
		return n.text
	}
	var b strings.Builder
	b.Grow(n.length)
	collect(n, &b)
	return b.String()
}

// splitAt splits the subtree rooted at n at byte offset i.
// Returns (left, right) where left contains bytes [0,i) and right contains [i,length).
func splitAt(n *ropeNode, i int) (*ropeNode, *ropeNode) {
	if n == nil {
		return nil, nil
	}
	if i <= 0 {
		return nil, n
	}
	if i >= n.length {
		return n, nil
	}
	if n.isLeaf() {
		// Split the leaf text at byte position i.
		return newLeaf(n.text[:i]), newLeaf(n.text[i:])
	}
	leftLen := 0
	if n.left != nil {
		leftLen = n.left.length
	}
	if i < leftLen {
		// Split falls within left subtree.
		ll, lr := splitAt(n.left, i)
		return ll, concat(lr, n.right)
	} else if i == leftLen {
		return n.left, n.right
	} else {
		// Split falls within right subtree.
		rl, rr := splitAt(n.right, i-leftLen)
		return concat(n.left, rl), rr
	}
}

// buildFromString builds a balanced rope tree from s by recursively halving.
func buildFromString(s string) *ropeNode {
	if len(s) == 0 {
		return nil
	}
	if len(s) <= leafMaxSize {
		return newLeaf(s)
	}
	mid := len(s) / 2
	return concat(buildFromString(s[:mid]), buildFromString(s[mid:]))
}

// Rope is a persistent text buffer backed by a balanced binary tree.
type Rope struct {
	root *ropeNode
}

// NewRope creates a new Rope from the given string.
func NewRope(s string) *Rope {
	return &Rope{root: buildFromString(s)}
}

// Len returns the total byte length of the rope's content.
func (r *Rope) Len() int {
	if r.root == nil {
		return 0
	}
	return r.root.length
}

// lineCountOf returns the logical line count for the rope's content.
// This matches the test expectations:
//
//	""              -> 0
//	"no newline"    -> 1  (partial line)
//	"one\n"         -> 1  (one complete line)
//	"one\ntwo\n"    -> 2
//	"one\ntwo\nthree" -> 3 (two complete + one partial)
func (r *Rope) LineCount() int {
	if r.root == nil {
		return 0
	}
	newlines := r.root.lineCount
	if newlines == 0 {
		// Non-empty content with no newlines: one partial line.
		if r.root.length > 0 {
			return 1
		}
		return 0
	}
	s := nodeString(r.root)
	if len(s) > 0 && s[len(s)-1] != '\n' {
		// Trailing partial line.
		return newlines + 1
	}
	return newlines
}

// String returns the full content of the rope as a string.
func (r *Rope) String() string {
	return nodeString(r.root)
}

// Insert inserts text at the given byte offset.
func (r *Rope) Insert(offset int, text string) {
	left, right := splitAt(r.root, offset)
	ins := buildFromString(text)
	r.root = concat(concat(left, ins), right)
}

// Delete removes bytes in the range [start, end).
// start and end are byte offsets; end is exclusive.
func (r *Rope) Delete(start, end int) {
	left, rest := splitAt(r.root, start)
	_, right := splitAt(rest, end-start)
	r.root = concat(left, right)
}

// Slice returns the substring of the rope between byte offsets [start, end).
func (r *Rope) Slice(start, end int) string {
	_, right := splitAt(r.root, start)
	left, _ := splitAt(right, end-start)
	return nodeString(left)
}

// LineAt returns the full text of the given zero-based line, including its newline (if any).
func (r *Rope) LineAt(line int) string {
	start := r.OffsetForLine(line)
	s := nodeString(r.root)
	if start >= len(s) {
		return ""
	}
	end := strings.IndexByte(s[start:], '\n')
	if end == -1 {
		return s[start:]
	}
	return s[start : start+end+1]
}

// OffsetForLine returns the byte offset of the start of the given zero-based line.
func (r *Rope) OffsetForLine(line int) int {
	if line == 0 {
		return 0
	}
	return offsetForLine(r.root, line)
}

// offsetForLine walks the tree to find the byte offset of the start of the given line.
// It counts newlines using the cached lineCount metadata on internal nodes.
func offsetForLine(n *ropeNode, targetLine int) int {
	if n == nil {
		return 0
	}
	if n.isLeaf() {
		// Walk the leaf text counting newlines.
		count := 0
		for i := 0; i < len(n.text); i++ {
			if n.text[i] == '\n' {
				count++
				if count == targetLine {
					return i + 1
				}
			}
		}
		// Target line not found in this leaf; return length (caller handles).
		return n.length
	}
	leftLines := 0
	if n.left != nil {
		leftLines = n.left.lineCount
	}
	if targetLine <= leftLines {
		// The target line starts within the left subtree.
		return offsetForLine(n.left, targetLine)
	}
	// The target line is in the right subtree; adjust by left length + remaining lines.
	leftLen := 0
	if n.left != nil {
		leftLen = n.left.length
	}
	return leftLen + offsetForLine(n.right, targetLine-leftLines)
}

// LineColForOffset returns the zero-based line and column for a given byte offset.
func (r *Rope) LineColForOffset(offset int) (line, col int) {
	s := nodeString(r.root)
	if offset > len(s) {
		offset = len(s)
	}
	sub := s[:offset]
	line = strings.Count(sub, "\n")
	lastNL := strings.LastIndexByte(sub, '\n')
	if lastNL == -1 {
		col = offset
	} else {
		col = offset - lastNL - 1
	}
	return line, col
}
