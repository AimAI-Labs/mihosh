package sync

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

type postgresTestStep struct {
	query string
	err   error
	row   []driver.Value
}

type postgresTestConnection struct{ steps []postgresTestStep }

func (c *postgresTestConnection) Connect(context.Context) (driver.Conn, error) { return c, nil }
func (*postgresTestConnection) Driver() driver.Driver                          { return nil }
func (*postgresTestConnection) Close() error                                   { return nil }
func (*postgresTestConnection) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected database transaction")
}
func (*postgresTestConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (c *postgresTestConnection) next(query string) (postgresTestStep, error) {
	if len(c.steps) == 0 {
		return postgresTestStep{}, fmt.Errorf("unexpected SQL: %s", query)
	}
	step := c.steps[0]
	c.steps = c.steps[1:]
	if step.query != query {
		return postgresTestStep{}, fmt.Errorf("unexpected SQL: %s, want %s", query, step.query)
	}
	return step, step.err
}

func (c *postgresTestConnection) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	_, err := c.next(query)
	return driver.RowsAffected(1), err
}

func (c *postgresTestConnection) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	step, err := c.next(query)
	if err != nil {
		return nil, err
	}
	return &postgresTestRows{row: step.row}, nil
}

type postgresTestRows struct {
	row  []driver.Value
	read bool
}

func (r *postgresTestRows) Columns() []string { return make([]string, len(r.row)) }
func (*postgresTestRows) Close() error        { return nil }
func (r *postgresTestRows) Next(dest []driver.Value) error {
	if r.read {
		return io.EOF
	}
	copy(dest, r.row)
	r.read = true
	return nil
}

func newPostgresTestStore(t *testing.T, steps ...postgresTestStep) *PostgresStore {
	t.Helper()
	conn := &postgresTestConnection{steps: steps}
	store := &PostgresStore{db: sql.OpenDB(conn)}
	t.Cleanup(func() {
		require.NoError(t, store.Close())
		require.Empty(t, conn.steps, "unused SQL expectations")
	})
	return store
}

func TestPostgresPushUsesDDLOnlyForMissingSchema(t *testing.T) {
	write := postgresTestStep{query: upsertSyncSnapshotSQL}
	for _, tc := range []struct {
		name  string
		steps []postgresTestStep
	}{
		{"ready", []postgresTestStep{write}},
		{"absent table", []postgresTestStep{
			{query: upsertSyncSnapshotSQL, err: &pq.Error{Code: "42P01"}},
			{query: createSyncTableSQL},
			write,
		}},
		{"legacy table", []postgresTestStep{
			{query: upsertSyncSnapshotSQL, err: &pq.Error{Code: "42703"}},
			{query: addSyncContentSHAColumnSQL},
			write,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newPostgresTestStore(t, tc.steps...)
			require.NoError(t, store.Push(context.Background(), Snapshot{Payload: []byte("test")}))
		})
	}
}

func TestPostgresReadsDoNotRequireDDL(t *testing.T) {
	ctx := context.Background()
	payload := []byte("fixture")
	for _, useStatus := range []bool{false, true} {
		query, name := selectSyncSnapshotSQL, "pull"
		var second driver.Value = payload
		read := func(store *PostgresStore) (SnapshotMeta, error) {
			snap, err := store.Pull(ctx, "default")
			return snap.Meta, err
		}
		if useStatus {
			query, name = selectSyncStatusSQL, "status"
			second = int64(len(payload))
			read = func(store *PostgresStore) (SnapshotMeta, error) { return store.Status(ctx, "default") }
		}
		t.Run(name+"/ready", func(t *testing.T) {
			row := []driver.Value{"default", second, PayloadSHA256(payload), "content", "", "", true, time.Now()}
			meta, err := read(newPostgresTestStore(t, postgresTestStep{query: query, row: row}))
			require.NoError(t, err)
			require.Equal(t, "content", meta.ContentSHA)
		})
		t.Run(name+"/legacy", func(t *testing.T) {
			row := []driver.Value{"default", second, PayloadSHA256(payload), "", "", "", true, time.Now()}
			store := newPostgresTestStore(t,
				postgresTestStep{query: query, err: &pq.Error{Code: "42703"}},
				postgresTestStep{query: strings.Replace(query, "content_sha256", "''", 1), row: row})
			meta, err := read(store)
			require.NoError(t, err)
			require.Empty(t, meta.ContentSHA)
		})
		t.Run(name+"/absent", func(t *testing.T) {
			_, err := read(newPostgresTestStore(t, postgresTestStep{query: query, err: &pq.Error{Code: "42P01"}}))
			require.ErrorIs(t, err, ErrSnapshotNotFound)
		})
	}
}
