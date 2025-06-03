package main

import (
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"
)

// Purpose is to reliably extract columns names
// Does so by parsing the query rune by rune to tokenise it into identifiers and some special characters
// Single and backtick quoted identifiers are handled
// 20% faster than the regexp solution
// Benchmark results:
// BenchmarkParse-8          849250              1215 ns/op
// BenchmarkRegexp-8         713101              1499 ns/op
// It handles case where table name and the opening parenthesis are not separated by a space https://github.com/ClickHouse/clickhouse-go/issues/1485#issuecomment-2632413186
// It handles case where a space preceeds a opening parenthesis in a quoted column name
type columnExtractor struct {
	query     string
	currToken []rune
	tokens    []string
	byteIndex int
}

// Pre-allocate a map for faster character lookups
var validIdentifierChars = make(map[rune]bool)

func init() {
	// Initialize the map with valid identifier characters
	for _, r := range "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_" {
		validIdentifierChars[r] = true
	}
}

func (e *columnExtractor) parseUntilClosingBackTick() ([]rune, error) {
	if len(e.query) == e.byteIndex {
		return nil, fmt.Errorf("unclosed backtick quote")
	}
	runeValue, width := utf8.DecodeRuneInString(e.query[e.byteIndex:])
	e.byteIndex += width
	e.currToken = append(e.currToken, runeValue)
	if runeValue == '`' && e.currToken[len(e.currToken)-2] != '\\' {
		return e.currToken, nil
	}
	return e.parseUntilClosingBackTick()
}

func (e *columnExtractor) parseUntilClosingSingleQuote() ([]rune, error) {
	if len(e.query) == e.byteIndex {
		return nil, fmt.Errorf("unclosed single quote")
	}
	runeValue, width := utf8.DecodeRuneInString(e.query[e.byteIndex:])
	e.byteIndex += width
	e.currToken = append(e.currToken, runeValue)
	if runeValue == '\'' && e.currToken[len(e.currToken)-2] != '\\' {
		return e.currToken, nil
	}
	return e.parseUntilClosingSingleQuote()
}

func (e *columnExtractor) parseNonQuotedIdentifier() ([]rune, error) {
	if len(e.query) == e.byteIndex {
		return e.currToken, nil
	}
	runeValue, width := utf8.DecodeRuneInString(e.query[e.byteIndex:])
	if !validIdentifierChars[runeValue] {
		return e.currToken, nil
	}
	e.byteIndex += width
	e.currToken = append(e.currToken, runeValue)
	return e.parseNonQuotedIdentifier()
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n'
}

func (e *columnExtractor) parse() error {
	// Pre-allocate tokens slice with a reasonable capacity
	e.tokens = make([]string, 0, len(e.query)/4) // Estimate 4 chars per token
	e.currToken = make([]rune, 0, 32)            // Pre-allocate for typical token size

	errs := make([]error, 0, 4) // Pre-allocate error slice

	for e.byteIndex < len(e.query) {
		runeValue, width := utf8.DecodeRuneInString(e.query[e.byteIndex:])
		e.byteIndex += width

		if isSpace(runeValue) {
			continue
		}

		switch runeValue {
		case '`':
			e.currToken = append(e.currToken[:0], runeValue) // Reset slice
			token, err := e.parseUntilClosingBackTick()
			if err != nil {
				errs = append(errs, err)
			}
			e.tokens = append(e.tokens, string(token))
		case '\'':
			e.currToken = append(e.currToken[:0], runeValue) // Reset slice
			token, err := e.parseUntilClosingSingleQuote()
			if err != nil {
				errs = append(errs, err)
			}
			e.tokens = append(e.tokens, string(token))
		case '(', ')', ',', '.':
			e.tokens = append(e.tokens, string(runeValue))
		default:
			if validIdentifierChars[runeValue] {
				e.currToken = append(e.currToken[:0], runeValue) // Reset slice
				token, err := e.parseNonQuotedIdentifier()
				if err != nil {
					errs = append(errs, err)
				}
				e.tokens = append(e.tokens, string(token))
			} else {
				errs = append(errs, fmt.Errorf(`unexpected rune: %s`, string(runeValue)))
			}
		}
	}
	return errors.Join(errs...)
}

func (e *columnExtractor) columns() []string {
	// Pre-allocate columns slice with a reasonable capacity
	columns := make([]string, 0, len(e.tokens)/2)
	openingParenthesisObserved := false

	for _, token := range e.tokens {
		switch token {
		case "(":
			openingParenthesisObserved = true
		case ")":
			return columns
		default:
			if openingParenthesisObserved && token != "," {
				columns = append(columns, token)
			}
		}
	}
	return columns
}

func main() {
	queries := []string{
		"INSERT INTO `DATA (BASE`.`A (TABLE)` ( `column \\`one`, columnTwo, 'col)umn\\' (three ')",
		"INSERT INTO db.table (`ITEM`, `QTY (MT)`)",
	}

	for _, query := range queries {
		extractor := &columnExtractor{
			query: query,
		}
		err := extractor.parse()
		if err != nil {
			panic(err)
		}
		columns := extractor.columns()
		fmt.Println("parser based", query, columns)

		matches := extractInsertColumnsMatch.FindStringSubmatch(query)

		fmt.Println("regexp based", query, matches[1])
	}
}

// copied from clickhouse-go source code
var extractInsertColumnsMatch = regexp.MustCompile(`(?si)INSERT INTO .+\s\((?P<Columns>.+)\)$`)
