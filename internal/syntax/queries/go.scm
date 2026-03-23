(package_clause (package_identifier) @module)
(import_spec path: (interpreted_string_literal) @string)
["break" "case" "chan" "const" "continue" "default" "defer" "else" "fallthrough" "for" "func" "go" "goto" "if" "import" "interface" "map" "package" "range" "return" "select" "struct" "switch" "type" "var"] @keyword
(function_declaration name: (identifier) @function)
(method_declaration name: (field_identifier) @function)
(call_expression function: (identifier) @function.call)
(call_expression function: (selector_expression field: (field_identifier) @function.call))
(type_declaration (type_spec name: (type_identifier) @type))
(type_identifier) @type
(const_declaration (const_spec name: (identifier) @constant))
(comment) @comment
(interpreted_string_literal) @string
(raw_string_literal) @string
(rune_literal) @string
(int_literal) @number
(float_literal) @number
(imaginary_literal) @number
(identifier) @variable
["+" "-" "*" "/" "%" "&" "|" "^" "<<" ">>" "+=" "-=" "*=" "/=" "%=" "&=" "|=" "^=" "==" "!=" "<" ">" "<=" ">=" "&&" "||" "!" "=" ":=" "<-"] @operator
["(" ")" "[" "]" "{" "}"] @punctuation
["," "." ";" ":"] @punctuation
