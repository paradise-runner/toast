; R syntax highlighting for internal/syntax.
;
; Deliberately predicate-free: the highlighter calls QueryCursor.Exec followed
; by NextMatch, which do not evaluate query predicates. A pattern guarded by
; (#eq? ...) would therefore match unconditionally. Upstream
; tree-sitter-r/highlights.scm uses #eq? for return(); that pattern is omitted
; here rather than mis-highlighting every call.
;
; Capture names are stripped at the first '.' and looked up in the theme's
; syntax map, falling back to the foreground for unknown names, so dotted
; captures such as @keyword.function are safe.

; ── Literals ────────────────────────────────────────────────
(integer) @number
(float) @number
(complex) @number
(string) @string

; ── Comments ────────────────────────────────────────────────
(comment) @comment

; ── Keywords and constants ──────────────────────────────────
["if" "else" "while" "repeat" "for" "in"] @keyword
(function_definition name: "function" @keyword)
[(next) (break)] @keyword
[(true) (false)] @constant
[(null) (inf) (nan) (na) (dots) (dot_dot_i)] @constant

; ── Assignments, functions, namespaces ──────────────────────
(binary_operator lhs: (identifier) @function operator: "<-" rhs: (function_definition))
(binary_operator lhs: (identifier) @function operator: "=" rhs: (function_definition))
(call function: (identifier) @function)
(namespace_operator lhs: (identifier) @module)
(call function: (namespace_operator rhs: (identifier) @function))

; ── Operators ───────────────────────────────────────────────
["?" ":=" "=" "<-" "<<-" "->" "->>" "~" "|>" "||" "|" "&&" "&"
 "<" "<=" ">" ">=" "==" "!=" "+" "-" "*" "/" "::" ":::" "**" "^"
 "$" "@" ":" "!" "special"] @operator

; ── Punctuation ─────────────────────────────────────────────
["(" ")" "{" "}" "[" "]" "[[" "]]"] @punctuation
(comma) @punctuation

; ── Fallback ────────────────────────────────────────────────
; Kept last so the more specific captures above win when equal-start spans are
; rendered in query order.
(identifier) @variable
