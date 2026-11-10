; Elixir highlighting.
;
; The tree-sitter-elixir grammar vendored in smacker/go-tree-sitter predates
; `module_attribute` nodes, so `def`/`defp`/`defmodule`/`alias`/`case` arrive
; as plain `(identifier)` call targets and cannot be told apart structurally.
; They are captured as @keyword and then filtered by LangDef.Keywords, which
; drops any identifier that is not a real Elixir keyword.

(comment) @comment

(string) @string
(charlist) @string
(char) @string
(sigil) @string
(sigil_name) @attribute

(atom) @constant
(quoted_atom) @constant
(boolean) @constant
(nil) @constant
(integer) @number
(float) @number

(alias) @type
(struct) @type

(keyword) @property
(pair key: (keyword) @property)
(dot left: (identifier) right: (identifier) @property)
(dot left: (alias) right: (identifier) @function.call)

(call target: (identifier) @keyword)

["do" "end" "fn" "when" "and" "or" "not" "in" "after" "else" "rescue" "catch"] @keyword

["+" "-" "*" "/" "++" "--" "<>" "=" "==" "!=" "===" "!==" "|>" "<-" "->" "=>" "::" "|" "&&" "||" "!" ".."] @operator
["(" ")" "[" "]" "{" "}" "<<" ">>" "%" "@"] @punctuation
["," ";" ":"] @punctuation
