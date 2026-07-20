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

// nodeLength returns n.length handling nil.
func nodeLength(n *ropeNode) int {
	if n == nil {
		return 0
	}
	return n.length
}

// byteAt returns the byte at the given absolute byte offset (relative to the
// subtree rooted at n) by descending the tree in O(log n). Returns 0 for an
// out-of-range offset (which is a valid byte value only for empty content;
// callers guard against empty trees before relying on the result).
func byteAt(n *ropeNode, offset int) byte {
	for n != nil {
		if n.isLeaf() {
			if offset >= 0 && offset < len(n.text) {
				return n.text[offset]
			}
			return 0
		}
		leftLen := nodeLength(n.left)
		if offset < leftLen {
			n = n.left
		} else {
			offset -= leftLen
			n = n.right
		}
	}
	return 0
}

// collectSubstring appends the bytes of subtree n in the byte range [start, end)
// (offsets relative to n) to buf. It descends only the subtrees that intersect
// the requested range, so it is O(log n + (end-start)).
func collectSubstring(n *ropeNode, start, end int, buf *strings.Builder) {
	if n == nil || start >= end || start >= n.length || end <= 0 {
		return
	}
	if n.isLeaf() {
		if start < 0 {
			start = 0
		}
		if end > n.length {
			end = n.length
		}
		buf.WriteString(n.text[start:end])
		return
	}
	leftLen := nodeLength(n.left)
	if start < leftLen {
		collectSubstring(n.left, start, min(end, leftLen), buf)
	}
	if end > leftLen {
		rs := start - leftLen
		if rs < 0 {
			rs = 0
		}
		collectSubstring(n.right, rs, end-leftLen, buf)
	}
}

// indexByteFrom returns the absolute byte offset (relative to subtree n) of
// the first '\n' at or after offset, or -1 if none. Descends in O(log n + the
// distance to the next newline within the leaf), matching strings.IndexByte on
// a single concatenated string without materializing it.
func indexByteFrom(n *ropeNode, offset int) int {
	if n == nil || offset >= n.length {
		return -1
	}
	if n.isLeaf() {
		if offset < 0 {
			offset = 0
		}
		if idx := strings.IndexByte(n.text[offset:], '\n'); idx >= 0 {
			return offset + idx
		}
		return -1
	}
	leftLen := nodeLength(n.left)
	if offset < leftLen {
		if idx := indexByteFrom(n.left, offset); idx >= 0 {
			return idx
		}
	}
	rs := offset - leftLen
	if rs < 0 {
		rs = 0
	}
	if idx := indexByteFrom(n.right, rs); idx >= 0 {
		return leftLen + idx
	}
	return -1
}

// countNewlinesBefore returns the number of '\n' bytes in subtree n within the
// byte range [0, offset) (offset relative to n). Uses cached lineCount on
// internal nodes, so it is O(log n + leafSize).
func countNewlinesBefore(n *ropeNode, offset int) int {
	if n == nil || offset <= 0 {
		return 0
	}
	if offset > n.length {
		offset = n.length
	}
	if n.isLeaf() {
		return strings.Count(n.text[:offset], "\n")
	}
	leftLen := nodeLength(n.left)
	if offset <= leftLen {
		return countNewlinesBefore(n.left, offset)
	}
	return n.left.lineCount + countNewlinesBefore(n.right, offset-leftLen)
}

// lastNewlineBefore returns the absolute byte offset (relative to subtree n) of
// the last '\n' strictly before offset, or -1 if none. O(log n + leafSize).
func lastNewlineBefore(n *ropeNode, offset int) int {
	if n == nil || offset <= 0 {
		return -1
	}
	if n.isLeaf() {
		if offset > n.length {
			offset = n.length
		}
		return strings.LastIndexByte(n.text[:offset], '\n')
	}
	leftLen := nodeLength(n.left)
	if offset <= leftLen {
		return lastNewlineBefore(n.left, offset)
	}
	if idx := lastNewlineBefore(n.right, offset-leftLen); idx >= 0 {
		return leftLen + idx
	}
	return lastNewlineBefore(n.left, leftLen)
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
//
// LineCount returns the logical number of lines for the rope's content.
// This matches the test expectations:
//
//	""                -> 0
//	"no newline"      -> 1  (partial line)
//	"one\n"           -> 1  (one complete line)
//	"one\ntwo\n"      -> 2
//	"one\ntwo\nthree" -> 3 (two complete + one partial)
//
// This used to call nodeString(root) just to inspect the final byte,
// materializing the entire buffer string on every call. It now inspects the
// final byte in O(log n) via byteAt, so callers that poll LineCount on every
// cursor movement no longer pay O(total bytes) per query.
func (r *Rope) LineCount() int {
	if r.root == nil || r.root.length == 0 {
		return 0
	}
	newlines := r.root.lineCount
	if newlines == 0 {
		// Non-empty content with no newlines: one partial line.
		return 1
	}
	// A trailing partial line exists iff the final byte is not a newline.
	if byteAt(r.root, r.root.length-1) == '\n' {
		return newlines
	}
	return newlines + 1
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
	total := r.Len()
	if start < 0 {
		start = 0
	}
	if end > total {
		end = total
	}
	if start >= end {
		return ""
	}
	var buf strings.Builder
	buf.Grow(end - start)
	collectSubstring(r.root, start, end, &buf)
	return buf.String()
}

// LineAt returns the full text of the given zero-based line, including its newline (if any).
func (r *Rope) LineAt(line int) string {
	total := r.Len()
	if total == 0 {
		return ""
	}
	start := r.OffsetForLine(line)
	if start < 0 {
		start = 0
	}
	if start >= total {
		return ""
	}
	end := total
	if nl := indexByteFrom(r.root, start); nl >= 0 {
		end = nl + 1 // include the newline
	}
	var buf strings.Builder
	buf.Grow(end - start)
	collectSubstring(r.root, start, end, &buf)
	return buf.String()
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
	total := r.Len()
	if total == 0 {
		return 0, 0
	}
	if offset > total {
		offset = total
	}
	if offset < 0 {
		offset = 0
	}
	line = countNewlinesBefore(r.root, offset)
	lastNL := lastNewlineBefore(r.root, offset)
	if lastNL < 0 {
		col = offset
	} else {
		col = offset - lastNL - 1
	}
	return line, col
}
