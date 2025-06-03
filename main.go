package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Purpose is to reliably extract columns names
// Does so by parsing the query rune by rune to tokenise it into identifiers and some special characters
// Single and backtick quoted identifiers are handled
// 30% slower than the regexp solution
// Benchmark results:
// BenchmarkParse-8          525538              2023 ns/op
// BenchmarkRegexp-8         801073              1552 ns/op
// It handles case where table name and the opening parenthesis are not separated by a space https://github.com/ClickHouse/clickhouse-go/issues/1485#issuecomment-2632413186
// It handles case where a space preceeds a opening parenthesis in a quoted column name
type columnExtractor struct {
	query     string
	currToken []rune
	tokens    []string
	byteIndex int
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

var nonQuotedIdentifierRunes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"

func (e *columnExtractor) parseNonQuotedIdentifier() ([]rune, error) {
	if len(e.query) == e.byteIndex {
		return e.currToken, nil
	}
	runeValue, width := utf8.DecodeRuneInString(e.query[e.byteIndex:])
	if !strings.ContainsRune(nonQuotedIdentifierRunes, runeValue) {
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
	errs := []error{}
	for e.byteIndex < len(e.query) {
		runeValue, width := utf8.DecodeRuneInString(e.query[e.byteIndex:])
		e.byteIndex += width
		if isSpace(runeValue) {
		} else if runeValue == '`' {
			e.currToken = append(e.currToken, runeValue)
			token, err := e.parseUntilClosingBackTick()
			if err != nil {
				errs = append(errs, err)
			}
			e.currToken = []rune{}
			e.tokens = append(e.tokens, string(token))
		} else if runeValue == '\'' {
			e.currToken = append(e.currToken, runeValue)
			token, err := e.parseUntilClosingSingleQuote()
			if err != nil {
				errs = append(errs, err)
			}
			e.currToken = []rune{}
			e.tokens = append(e.tokens, string(token))
		} else if strings.ContainsRune(nonQuotedIdentifierRunes, runeValue) {
			e.currToken = append(e.currToken, runeValue)
			token, err := e.parseNonQuotedIdentifier()
			if err != nil {
				errs = append(errs, err)
			}
			e.currToken = []rune{}
			e.tokens = append(e.tokens, string(token))
		} else if runeValue == '(' {
			e.tokens = append(e.tokens, string(runeValue))
		} else if runeValue == ')' {
			e.tokens = append(e.tokens, string(runeValue))
		} else if runeValue == ',' {
			e.tokens = append(e.tokens, string(runeValue))
		} else if runeValue == '.' {
			e.tokens = append(e.tokens, string(runeValue))
		} else {
			errs = append(errs, fmt.Errorf(`unexpected rune: %s`, string(runeValue)))
		}
	}
	return errors.Join(errs...)
}

func (e *columnExtractor) columns() []string {
	openingParenthesisObserved := false
	closingParenthesisObserved := false
	columns := []string{}
	for _, token := range e.tokens {
		if token == "(" {
			openingParenthesisObserved = true
		} else if token == ")" {
			closingParenthesisObserved = true
			break
		} else if openingParenthesisObserved && !closingParenthesisObserved && token != "," {
			columns = append(columns, token)
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
