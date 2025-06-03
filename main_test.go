package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractInsertColumns(t *testing.T) {
	t.Run(`simple`, func(t *testing.T) {
		e := &columnExtractor{
			query: `INSERT INTO table (column1, column2)`,
		}
		err := e.parse()
		assert.NoError(t, err)
		t.Log(e.tokens)
		assert.Equal(t, 8, len(e.tokens))
		assert.Equal(t, `column1`, e.tokens[4])
		assert.Equal(t, `column2`, e.tokens[6])
	})

	t.Run(`table name with dots`, func(t *testing.T) {
		e := &columnExtractor{
			query: `INSERT INTO db.table (column1, column2)`,
		}
		err := e.parse()
		assert.NoError(t, err)
		assert.Equal(t, 10, len(e.tokens))
		assert.Equal(t, `db`, e.tokens[2])
		assert.Equal(t, `.`, e.tokens[3])
	})

	t.Run(`columns in single quotes`, func(t *testing.T) {
		e := &columnExtractor{
			query: `INSERT INTO table ('column 1')`,
		}
		err := e.parse()
		assert.NoError(t, err)
		assert.Equal(t, `'column 1'`, e.tokens[4])
	})

	t.Run(`columns in double backticks`, func(t *testing.T) {
		e := &columnExtractor{
			query: "INSERT INTO table (`column1`)",
		}
		err := e.parse()
		assert.NoError(t, err)
		assert.Equal(t, "`column1`", e.tokens[4])
	})

	t.Run(`column containing backticks and single quotes inside quoted backticks`, func(t *testing.T) {
		e := &columnExtractor{
			query: "INSERT INTO table (`colum\\`n1`, `colu'mn2`)",
		}
		err := e.parse()
		assert.NoError(t, err)
		assert.Equal(t, "`colum\\`n1`", e.tokens[4])
		assert.Equal(t, "`colu'mn2`", e.tokens[6])
	})

	t.Run(`parentheses inside column names`, func(t *testing.T) {
		e := &columnExtractor{
			query: "INSERT INTO table (`WEIGHT (kg)` )",
		}
		err := e.parse()
		assert.NoError(t, err)
		assert.Equal(t, "`WEIGHT (kg)`", e.tokens[4])
	})

	t.Run(`comma inside column names`, func(t *testing.T) {
		e := &columnExtractor{
			query: "INSERT INTO table (`WEIGHT, in kg` )",
		}
		err := e.parse()
		assert.NoError(t, err)
		assert.Equal(t, "`WEIGHT, in kg`", e.tokens[4])
	})

	t.Run(`columns`, func(t *testing.T) {
		e := &columnExtractor{
			query: "INSERT INTO table (`WEIGHT, in kg`, 'height in cm.')",
		}
		err := e.parse()
		assert.NoError(t, err)
		columns := e.columns()
		assert.Contains(t, columns, "`WEIGHT, in kg`")
		assert.Contains(t, columns, "'height in cm.'")
	})

	t.Run(`without space between table name and parentheses`, func(t *testing.T) {
		e := &columnExtractor{
			query: "INSERT INTO table(column1, column2)",
		}
		err := e.parse()
		assert.NoError(t, err)
		assert.Equal(t, 8, len(e.tokens))
		assert.Equal(t, `column1`, e.tokens[4])
		assert.Equal(t, `column2`, e.tokens[6])
	})
}

func BenchmarkParse(b *testing.B) {
	query := `INSERT INTO table (column1, column2)`
	for i := 0; i < b.N; i++ {
		e := &columnExtractor{
			query: query,
		}
		err := e.parse()
		e.columns()
		assert.NoError(b, err)
	}
}

func BenchmarkRegexp(b *testing.B) {
	query := `INSERT INTO table (column1, column2)`
	for i := 0; i < b.N; i++ {
		matches := extractInsertColumnsMatch.FindStringSubmatch(query)
		assert.Equal(b, 2, len(matches))
	}
}
