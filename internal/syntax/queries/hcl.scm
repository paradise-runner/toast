; HCL / Terraform highlights

(comment) @comment

(string_lit) @string
(quoted_template) @string
(heredoc_template) @string

(numeric_lit) @number
(bool_lit) @constant
(null_lit) @constant

(variable_expr (identifier) @variable)

(function_call (identifier) @function.call)

(block
  (identifier) @keyword)

(attribute (identifier) @attribute)

[
  "==" "!=" "<" ">" "<=" ">="
  "+" "-" "*" "/" "%"
  "&&" "||" "!"
  "?" ":"
  "="
] @operator

[
  "(" ")" "[" "]" "{" "}"
] @punctuation

[
  ","
  "."
] @punctuation

(ellipsis) @punctuation

(for_intro
  "for" @keyword
  "in" @keyword)

(for_cond
  "if" @keyword)

(conditional
  "?" @operator
  ":" @operator)
