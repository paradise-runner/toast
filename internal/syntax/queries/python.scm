["and" "as" "assert" "async" "await" "break" "class" "continue" "def" "del" "elif" "else" "except" "finally" "for" "from" "global" "if" "import" "in" "is" "lambda" "nonlocal" "not" "or" "pass" "raise" "return" "try" "while" "with" "yield"] @keyword
(function_definition name: (identifier) @function)
(class_definition name: (identifier) @type)
(call function: (identifier) @function.call)
(call function: (attribute attribute: (identifier) @function.call))
(decorator) @attribute
(comment) @comment
(string) @string
(integer) @number
(float) @number
(true) @constant
(false) @constant
(none) @constant
(identifier) @variable
["+" "-" "*" "/" "//" "%" "**" "&" "|" "^" "~" "<<" ">>" "==" "!=" "<" ">" "<=" ">=" "=" ":="] @operator
["(" ")" "[" "]" "{" "}"] @punctuation
["," "." ";" ":" "->"] @punctuation
