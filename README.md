

## Purpose is to reliably extract columns names
- Does so by parsing the query rune by rune to tokenise it into identifiers and some special characters
- Single and backtick quoted identifiers are handled
- 20% faster than the regexp solution
- Benchmark results:

| Name              | ns/op      | Implenetation           |
|-------------------|------------|-------------------------|
| BenchmarkParse-8  | 1215 ns/op | From this repo          |
| BenchmarkRegexp-8 | 1499 ns/op | From clickhouse-go repo |

- It handles cases where table name and the opening parenthesis are not separated by a space https://github.com/ClickHouse/clickhouse-go/issues/1485#issuecomment-2632413186
- It handles cases where a space preceeds a opening parenthesis in a quoted column name


## Example
- Input: ```INSERT INTO `DATA (BASE`.`A (TABLE)` ( `column \`one`, columnTwo, 'col)umn\' (three ') ```
- Output: ```[`column \`one` , columnTwo , 'col)umn\' (three ']```
