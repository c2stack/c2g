module "module"
[ident] "scope"
{ "{"
prefix "prefix"
[string] "\"\""
; ";"
namespace "namespace"
[string] "\"\""
; ";"
revision "revision"
[string] "0"
; ";"
container "container"
[ident] "x"
{ "{"
grouping "grouping"
[ident] "g"
{ "{"
leaf "leaf"
[ident] "z"
{ "{"
type "type"
[ident] "string"
; ";"
} "}"
} "}"
container "container"
[ident] "y"
{ "{"
uses "uses"
[ident] "g"
; ";"
} "}"
} "}"
} "}"
