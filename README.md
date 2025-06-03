

## Purpose is to reliably extract columns names
- Does so by parsing the query rune by rune to tokenise it into identifiers and some special characters
- Single and backtick quoted identifiers are handled
- 25% slower than the regexp solution
- Benchmark results:
- BenchmarkParse-8          525538              2023 ns/op
- BenchmarkRegexp-8         801073              1552 ns/op
- It handles case where table name and the opening parenthesis are not separated by a space https://github.com/ClickHouse/clickhouse-go/issues/1485#issuecomment-2632413186
- It handles case where a space preceeds a opening parenthesis in a quoted column name