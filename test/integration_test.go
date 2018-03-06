package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	sqle "gopkg.in/src-d/go-mysql-server.v0"
	"gopkg.in/src-d/go-mysql-server.v0/sql"
)

func TestIntegration(t *testing.T) {
	engine := sqle.New()
	// TODO: create database and tables, insert known info for after check
	// engine.AddDatabase(..)

	for _, query := range queries {
		t.Run(query.name, func(t *testing.T) {
			require := require.New(t)
			schema, rowIter, err := engine.Query(query.statement)
			if query.expectedErr {
				require.Error(err)
			} else {
				require.NoError(err)
				checkSchema(t, schema, query.expectedSchema)
				checkIter(t, rowIter, query.expectedRows)
			}
		})
	}
}

func checkSchema(t *testing.T, schema, expected sql.Schema) {
	// TODO: add expected schema to the queries and check it
	require.Nil(t, expected)
}

func checkIter(t *testing.T, rowIter sql.RowIter, expected int) {
	// TODO: add expected number of rows to the queries and check it
	require.Zero(t, expected)
}
