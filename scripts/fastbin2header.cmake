# fastbin2header.cmake — Convert a binary file into a C header with a byte
# array, without the quadratic string(APPEND) loop of upstream ghostling's
# bin2header.cmake (which takes tens of minutes on multi-hundred-KB inputs).
#
# Drop-in replacement for ghostling's bin2header.cmake:
#   cmake -DINPUT=<file> -DOUTPUT=<file> -DARRAY_NAME=<name> -P fastbin2header.cmake
#
# A single regex pass rewrites the hex stream, keeping generation linear.

file(READ "${INPUT}" hex HEX)
string(REGEX REPLACE "([0-9a-f][0-9a-f])" "0x\\1, " out "${hex}")
string(REGEX REPLACE ", $" "" out "${out}")

set(output "// Auto-generated from ${INPUT} — do not edit.\n")
string(APPEND output "static const unsigned char ${ARRAY_NAME}[] = {\n    ")
string(APPEND output "${out}\n};\n")
file(WRITE "${OUTPUT}" "${output}")
