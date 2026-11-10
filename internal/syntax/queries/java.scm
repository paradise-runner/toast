; Keywords. Anonymous tokens only: boolean/void/this/super/true/false/null are
; named nodes in tree-sitter-java, and const/goto are reserved but never
; emitted, so listing any of them as a string would make the query fail to
; compile (and silently disable highlighting).
["abstract" "assert" "break" "byte" "case" "catch" "char" "class" "continue"
 "default" "do" "double" "else" "enum" "extends" "final" "finally" "float"
 "for" "if" "implements" "import" "instanceof" "int" "interface" "long"
 "native" "new" "package" "private" "protected" "public" "return" "short"
 "static" "strictfp" "switch" "synchronized" "throw" "throws"
 "transient" "try" "volatile" "while"] @keyword
(void_type) @keyword
(boolean_type) @keyword
(true) @constant
(false) @constant
(null_literal) @constant
(this) @constant
(super) @constant

; Declarations and calls
(class_declaration name: (identifier) @type)
(interface_declaration name: (identifier) @type)
(enum_declaration name: (identifier) @type)
(record_declaration name: (identifier) @type)
(annotation_type_declaration name: (identifier) @type)
(method_declaration name: (identifier) @function)
(constructor_declaration name: (identifier) @function)
(compact_constructor_declaration name: (identifier) @function)
(method_invocation name: (identifier) @function.call)
(object_creation_expression type: (type_identifier) @type)
(type_identifier) @type
(annotation name: (identifier) @attribute)
(marker_annotation name: (identifier) @attribute)

; Literals
(string_literal) @string
(character_literal) @string
(decimal_integer_literal) @number
(hex_integer_literal) @number
(octal_integer_literal) @number
(binary_integer_literal) @number
(decimal_floating_point_literal) @number
(hex_floating_point_literal) @number

; Comments
(line_comment) @comment
(block_comment) @comment

; Fallbacks. Keep these last so the more specific captures above win when the
; highlighter renders equal-start spans in query order.
(field_access field: (identifier) @property)
(identifier) @variable
