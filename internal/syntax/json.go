package syntax

// jsonToken represents a highlighted token in a JSON document.
// Start and End are byte offsets into the source content.
type jsonToken struct {
	startByte int
	endByte   int
	style     string
}

// scanJSON scans src and returns all highlight tokens for a JSON document.
// It uses a simple state machine to tokenize JSON and tracks object/array
// context so that object keys are highlighted with the "property" style
// while string values use the "string" style.
//
// For .jsonc files, both // and /* */ comments are recognised.
func scanJSON(src []byte, allowComments bool) []jsonToken {
	var tokens []jsonToken
	var ctxStack []bool // true = object, false = array
	expectKey := true   // at top level or after { / , inside object

	i := 0
	for i < len(src) {
		c := src[i]

		// ── Whitespace ─────────────────────────────────────────────
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}

		// ── Comments (JSONC) ───────────────────────────────────────
		if allowComments && c == '/' && i+1 < len(src) {
			if src[i+1] == '/' {
				start := i
				i += 2
				for i < len(src) && src[i] != '\n' {
					i++
				}
				tokens = append(tokens, jsonToken{start, i, "comment"})
				continue
			}
			if src[i+1] == '*' {
				start := i
				i += 2
				for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
					i++
				}
				if i+1 < len(src) {
					i += 2 // skip */
				}
				tokens = append(tokens, jsonToken{start, i, "comment"})
				continue
			}
		}

		// ── Structural characters ──────────────────────────────────
		if c == '{' {
			tokens = append(tokens, jsonToken{i, i + 1, "punctuation"})
			ctxStack = append(ctxStack, true)
			expectKey = true
			i++
			continue
		}
		if c == '}' {
			tokens = append(tokens, jsonToken{i, i + 1, "punctuation"})
			if len(ctxStack) > 0 {
				ctxStack = ctxStack[:len(ctxStack)-1]
			}
			expectKey = false
			i++
			continue
		}
		if c == '[' {
			tokens = append(tokens, jsonToken{i, i + 1, "punctuation"})
			ctxStack = append(ctxStack, false)
			expectKey = false
			i++
			continue
		}
		if c == ']' {
			tokens = append(tokens, jsonToken{i, i + 1, "punctuation"})
			if len(ctxStack) > 0 {
				ctxStack = ctxStack[:len(ctxStack)-1]
			}
			expectKey = false
			i++
			continue
		}
		if c == ':' {
			tokens = append(tokens, jsonToken{i, i + 1, "punctuation"})
			expectKey = false
			i++
			continue
		}
		if c == ',' {
			tokens = append(tokens, jsonToken{i, i + 1, "punctuation"})
			if len(ctxStack) > 0 && ctxStack[len(ctxStack)-1] {
				// Inside an object, comma means next item is a key.
				expectKey = true
			} else {
				expectKey = false
			}
			i++
			continue
		}

		// ── String literals ────────────────────────────────────────
		if c == '"' {
			start := i
			i++ // skip opening quote
			for i < len(src) {
				if src[i] == '\\' && i+1 < len(src) {
					i += 2 // skip escaped char
				} else if src[i] == '"' {
					i++ // skip closing quote
					break
				} else {
					i++
				}
			}
			style := "string"
			if expectKey {
				style = "property"
			}
			tokens = append(tokens, jsonToken{start, i, style})
			expectKey = false
			continue
		}

		// ── Numbers ────────────────────────────────────────────────
		if c == '-' || (c >= '0' && c <= '9') {
			start := i
			if c == '-' {
				i++
			}
			// integer part
			for i < len(src) && src[i] >= '0' && src[i] <= '9' {
				i++
			}
			// fractional part
			if i < len(src) && src[i] == '.' {
				i++
				for i < len(src) && src[i] >= '0' && src[i] <= '9' {
					i++
				}
			}
			// exponent
			if i < len(src) && (src[i] == 'e' || src[i] == 'E') {
				i++
				if i < len(src) && (src[i] == '+' || src[i] == '-') {
					i++
				}
				for i < len(src) && src[i] >= '0' && src[i] <= '9' {
					i++
				}
			}
			tokens = append(tokens, jsonToken{start, i, "number"})
			expectKey = false
			continue
		}

		// ── Keywords ───────────────────────────────────────────────
		if c == 't' && i+3 < len(src) && string(src[i:i+4]) == "true" {
			tokens = append(tokens, jsonToken{i, i + 4, "constant"})
			i += 4
			expectKey = false
			continue
		}
		if c == 'f' && i+4 < len(src) && string(src[i:i+5]) == "false" {
			tokens = append(tokens, jsonToken{i, i + 5, "constant"})
			i += 5
			expectKey = false
			continue
		}
		if c == 'n' && i+3 < len(src) && string(src[i:i+4]) == "null" {
			tokens = append(tokens, jsonToken{i, i + 4, "constant"})
			i += 4
			expectKey = false
			continue
		}

		// ── Unknown character – skip ───────────────────────────────
		i++
	}

	return tokens
}
