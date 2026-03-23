["if" "then" "else" "elif" "fi" "case" "esac" "for" "while" "do" "done" "in" "function" "select" "until" "local" "export"] @keyword
(function_definition name: (word) @function)
(command_name) @function.call
(comment) @comment
(string) @string
(raw_string) @string
(number) @number
(variable_name) @variable
["$" "${" "}" "(" ")" "[" "]" ";" "|" ">" "<" "&" "&&" "||"] @punctuation
