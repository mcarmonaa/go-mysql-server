package mem

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"strconv"

	errors "gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-mysql-server.v0/sql"
	"gopkg.in/src-d/go-mysql-server.v0/sql/expression"
)

// Table represents an in-memory database table.
type Table struct {
	name       string
	schema     sql.Schema
	partitions map[string][]sql.Row
	keys       [][]byte

	insert int

	Filters []sql.Expression
	Columns []int
	Lookup  sql.IndexLookup
}

var _ sql.Table = (*Table)(nil)
var _ sql.Inserter = (*Table)(nil)
var _ sql.FilteredTable = (*Table)(nil)
var _ sql.ProjectedTable = (*Table)(nil)
var _ sql.IndexableTable = (*Table)(nil)

// NewTable creates a new Table with the given name and schema.
func NewTable(name string, schema sql.Schema) *Table {
	return NewPartitionedTable(name, schema, 0)
}

// NewPartitionedTable creates a new Table with the given name, schema and number of partitions.
func NewPartitionedTable(name string, schema sql.Schema, numPartitions int) *Table {
	var keys [][]byte
	var partitions = map[string][]sql.Row{}

	if numPartitions < 1 {
		numPartitions = 1
	}

	for i := 0; i < numPartitions; i++ {
		key := strconv.Itoa(i)
		keys = append(keys, []byte(key))
		partitions[key] = []sql.Row{}
	}

	return &Table{
		name:       name,
		schema:     schema,
		partitions: partitions,
		keys:       keys,
	}
}

// Schema implements the sql.Table interface.
func (t *Table) Schema() sql.Schema {
	return t.schema
}

// Partitions implements the sql.Table interface.
func (t *Table) Partitions(ctx *sql.Context) (sql.PartitionIter, error) {
	return &partitionIter{keys: t.keys}, nil
}

// PartitionRows implements the sql.PartitionRows interface.
func (t *Table) PartitionRows(ctx *sql.Context, partition sql.Partition) (sql.RowIter, error) {
	key := string(partition.Key())
	rows, ok := t.partitions[key]
	if !ok {
		return nil, fmt.Errorf(
			"partition not found: %q", partition.Key(),
		)
	}

	var values sql.IndexValueIter
	if t.Lookup != nil {
		var err error
		values, err = t.Lookup.Values(partition)
		if err != nil {
			return nil, err
		}
	}

	return &tableIter{
		rows:        rows,
		columns:     t.Columns,
		filters:     t.Filters,
		indexValues: values,
	}, nil
}

type partition struct {
	key []byte
}

func (p *partition) Key() []byte { return p.key }

type partitionIter struct {
	keys [][]byte
	pos  int
}

func (p *partitionIter) Next() (sql.Partition, error) {
	if p.pos >= len(p.keys) {
		return nil, io.EOF
	}

	key := p.keys[p.pos]
	p.pos++
	return &partition{key}, nil
}

func (p *partitionIter) Close() error { return nil }

type tableIter struct {
	columns []int
	filters []sql.Expression

	rows        []sql.Row
	indexValues sql.IndexValueIter
	pos         int
}

var _ sql.RowIter = (*tableIter)(nil)

func (i *tableIter) Next() (sql.Row, error) {
	row, err := i.getRow()
	if err != nil {
		return nil, err
	}

	for _, f := range i.filters {
		result, err := f.Eval(sql.NewEmptyContext(), row)
		if err != nil {
			return nil, err
		}

		if result != true {
			return i.Next()
		}
	}

	return projectOnRow(i.columns, row), nil
}

func (i *tableIter) Close() error {
	if i.indexValues == nil {
		return nil
	}

	return i.indexValues.Close()
}

func (i *tableIter) getRow() (sql.Row, error) {
	if i.indexValues != nil {
		return i.getFromIndex()
	}

	if i.pos >= len(i.rows) {
		return nil, io.EOF
	}

	row := i.rows[i.pos]
	i.pos++
	return row, nil
}

func projectOnRow(columns []int, row sql.Row) sql.Row {
	if len(columns) < 1 {
		return row
	}

	projected := make([]interface{}, len(columns))
	for i, selected := range columns {
		projected[i] = row[selected]
	}

	return projected
}

func (i *tableIter) getFromIndex() (sql.Row, error) {
	data, err := i.indexValues.Next()
	if err != nil {
		return nil, err
	}

	value, err := decodeIndexValue(data)
	if err != nil {
		return nil, err
	}

	return i.rows[value.Pos], nil
}

type indexValue struct {
	Key string
	Pos int
}

func decodeIndexValue(data []byte) (*indexValue, error) {
	dec := gob.NewDecoder(bytes.NewReader(data))
	var value indexValue
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}

	return &value, nil
}

func encodeIndexValue(value *indexValue) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Insert a new row into the table.
func (t *Table) Insert(ctx *sql.Context, row sql.Row) error {
	if err := checkRow(t.schema, row); err != nil {
		return err
	}

	key := string(t.keys[t.insert])
	t.insert++
	if t.insert == len(t.keys) {
		t.insert = 0
	}

	t.partitions[key] = append(t.partitions[key], row)
	return nil
}

func checkRow(schema sql.Schema, row sql.Row) error {
	if len(row) != len(schema) {
		return sql.ErrUnexpectedRowLength.New(len(schema), len(row))
	}

	for i, value := range row {
		c := schema[i]
		if !c.Check(value) {
			return sql.ErrInvalidType.New(value)
		}
	}

	return nil
}

// String implements the sql.Table inteface.
func (t *Table) String() string {
	p := sql.NewTreePrinter()

	kind := ""
	if len(t.Columns) > 0 {
		kind += "Projected "
	}

	if len(t.Filters) > 0 {
		kind += "Filtered "
	}

	if t.Lookup != nil {
		kind += "Indexed"
	}

	if kind != "" {
		kind = ": " + kind
	}

	_ = p.WriteNode("Table(%s)%s", t.name, kind)
	var schema = make([]string, len(t.Schema()))
	for i, col := range t.Schema() {
		schema[i] = fmt.Sprintf(
			"Column(%s, %s, nullable=%v)",
			col.Name,
			col.Type.Type().String(),
			col.Nullable,
		)
	}
	_ = p.WriteChildren(schema...)
	return p.String()
}

// HandledFilters implements the sql.FilteredTable interface.
func (t *Table) HandledFilters(filters []sql.Expression) []sql.Expression {
	var handled []sql.Expression
	for _, f := range filters {
		var hasOtherFields bool
		f.TransformUp(func(e sql.Expression) (sql.Expression, error) {
			if e, ok := e.(*expression.GetField); ok {
				if e.Table() != t.name || !t.schema.Contains(e.Name(), t.name) {
					hasOtherFields = true
				}
			}

			return e, nil
		})

		if !hasOtherFields {
			handled = append(handled, f)
		}
	}

	return handled
}

// WithFilters implements the sql.FilteredTable interface.
func (t *Table) WithFilters(filters []sql.Expression) sql.Table {
	if len(filters) < 1 {
		return t
	}

	if t.Filters != nil {
		filters = append(filters, t.Filters...)
	}

	return &Table{
		name:       t.name,
		schema:     t.schema,
		partitions: t.partitions,
		keys:       t.keys,
		insert:     t.insert,
		Filters:    filters,
		Columns:    t.Columns,
		Lookup:     t.Lookup,
	}

}

// WithProjection implements the sql.ProjectedTable interface.
func (t *Table) WithProjection(colNames []string) sql.Table {
	if len(colNames) < 1 {
		return t
	}

	columns, schema, _ := t.newColumnIndexesAndSchema(colNames)
	return &Table{
		name:       t.name,
		schema:     schema,
		partitions: t.partitions,
		keys:       t.keys,
		insert:     t.insert,
		Filters:    t.Filters,
		Columns:    columns,
		Lookup:     t.Lookup,
	}
}

func (t *Table) newColumnIndexesAndSchema(colNames []string) ([]int, sql.Schema, error) {
	columns := []int{}
	schema := []*sql.Column{}

	for _, name := range colNames {
		i := t.schema.IndexOf(name, t.name)
		if i == -1 {
			return nil, nil, errColumnNotFound.New(name)
		}

		if len(t.Columns) == 0 {
			// if the table hasn't been projected before
			// match against the origianl schema
			columns = append(columns, i)
		} else {
			// get indexes for the new projections from
			// the orginal indexes.
			columns = append(columns, t.Columns[i])
		}

		schema = append(schema, t.schema[i])
	}

	return columns, schema, nil
}

// WithIndexLookup implements the sql.IndexableTable interface.
func (t *Table) WithIndexLookup(lookup sql.IndexLookup) sql.Table {
	if lookup == nil {
		return t
	}

	return &Table{
		name:       t.name,
		schema:     t.schema,
		partitions: t.partitions,
		keys:       t.keys,
		insert:     t.insert,
		Filters:    t.Filters,
		Columns:    t.Columns,
		Lookup:     lookup,
	}
}

// IndexKeyValues implements the sql.IndexableTable interface.
func (t *Table) IndexKeyValues(
	ctx *sql.Context,
	colNames []string,
	partition sql.Partition,
) (sql.IndexKeyValueIter, error) {
	rowIter, err := t.PartitionRows(ctx, partition)
	if err != nil {
		return nil, err
	}

	columns, _, err := t.newColumnIndexesAndSchema(colNames)
	if err != nil {
		return nil, err
	}

	return &idxKVIter{
		key:     string(partition.Key()),
		iter:    rowIter,
		columns: columns,
	}, nil
}

var errColumnNotFound = errors.NewKind("could not find column %s")

type idxKVIter struct {
	key     string
	iter    sql.RowIter
	columns []int
	pos     int
}

func (i *idxKVIter) Next() ([]interface{}, []byte, error) {
	row, err := i.iter.Next()
	if err != nil {
		return nil, nil, err
	}

	value := &indexValue{Key: i.key, Pos: i.pos}
	data, err := encodeIndexValue(value)
	if err != nil {
		return nil, nil, err
	}

	i.pos++
	return projectOnRow(i.columns, row), data, nil
}

func (i *idxKVIter) Close() error {
	return i.iter.Close()
}
