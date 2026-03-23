["async" "await" "break" "case" "catch" "class" "const" "continue" "debugger" "default" "delete" "do" "else" "export" "extends" "finally" "for" "from" "function" "if" "import" "in" "instanceof" "let" "new" "of" "return" "static" "switch" "throw" "try" "typeof" "var" "void" "while" "with" "yield"] @keyword
(function_declaration name: (identifier) @function)
(method_definition name: (property_identifier) @function)
(call_expression function: (identifier) @function.call)
(call_expression function: (member_expression property: (property_identifier) @function.call))
(class_declaration name: (identifier) @type)
(comment) @comment
(string) @string
(template_string) @string
(number) @number
(true) @constant
(false) @constant
(null) @constant
(undefined) @constant
(identifier) @variable
(property_identifier) @property
["+" "-" "*" "/" "%" "**" "==" "!=" "===" "!==" "<" ">" "<=" ">=" "&&" "||" "!" "=" "=>"] @operator
["(" ")" "[" "]" "{" "}"] @punctuation
["," "." ";" ":"] @punctuation
