# Style configuration for Kepler documentation files.
#
# For details on how to configure, see [markdownlint docs][1].
#
# For explanation of the rules themselves, see [markdownlint's RULES.md][2]
#
# [1]: https://github.com/markdownlint/markdownlint/blob/master/docs/creating_styles.md
# [2]: https://github.com/markdownlint/markdownlint/blob/master/docs/RULES.md

tag :whitespace
tag :headers
tag :ul
tag :indentation
tag :bullet

rule "ul-indent", indent: 4

rule "ul-style", style: :dash
rule "no-duplicate-header", allow_different_nesting: true
rule "line-length", :line_length => 120

exclude_rule 'MD026'           # Trailing punctuation in header
exclude_rule 'MD041'           # First line in file should be a top level header
exclude_rule 'MD007'           # Unordered list indentation
exclude_rule 'MD013'           # Line length
