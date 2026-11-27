package sync

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var (
	// ErrSnapshotNotFound indicates no remote sync snapshot exists for the given profile key.
	ErrSnapshotNotFound = errors.New("remote sync snapshot not found")
	// ErrIntegrityMismatch indicates the downloaded payload hash does not match stored metadata.
	ErrIntegrityMismatch = errors.New("remote sync snapshot checksum mismatch")
)

// SnapshotMeta describes a remote sync snapshot stored in PostgreSQL.
type SnapshotMeta struct {
	ProfileKey string    `json:"profile_key"`
	SHA256     string    `json:"sha256"`
	ContentSHA string    `json:"content_sha256,omitempty"`
	Device     string    `json:"device"`
	AppVersion string    `json:"app_version"`
	Redacted   bool      `json:"redacted"`
	SizeBytes  int       `json:"size_bytes"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Snapshot carries both snapshot metadata and the compressed archive payload.
type Snapshot struct {
	Meta    SnapshotMeta
	Payload []byte
}

// Store abstracts PostgreSQL cloud sync persistence for production and offline testing.
type Store interface {
	Push(ctx context.Context, snap Snapshot) error
	Pull(ctx context.Context, profileKey string) (Snapshot, error)
	Status(ctx context.Context, profileKey string) (SnapshotMeta, error)
	Close() error
}

const createSyncTableSQL = `
CREATE TABLE IF NOT EXISTS mihosh_sync_snapshots (
	profile_key TEXT PRIMARY KEY,
	payload BYTEA NOT NULL,
	sha256 TEXT NOT NULL,
	content_sha256 TEXT NOT NULL DEFAULT '',
	device TEXT NOT NULL DEFAULT '',
	app_version TEXT NOT NULL DEFAULT '',
	redacted BOOLEAN NOT NULL DEFAULT TRUE,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const addSyncContentSHAColumnSQL = `ALTER TABLE mihosh_sync_snapshots ADD COLUMN IF NOT EXISTS content_sha256 TEXT NOT NULL DEFAULT ''`

const (
	postgresUndefinedTable  = "42P01"
	postgresUndefinedColumn = "42703"
)

const upsertSyncSnapshotSQL = `
INSERT INTO mihosh_sync_snapshots (profile_key, payload, sha256, content_sha256, device, app_version, redacted, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (profile_key) DO UPDATE SET
	payload = EXCLUDED.payload,
	sha256 = EXCLUDED.sha256,
	content_sha256 = EXCLUDED.content_sha256,
	device = EXCLUDED.device,
	app_version = EXCLUDED.app_version,
	redacted = EXCLUDED.redacted,
	updated_at = EXCLUDED.updated_at
`

const selectSyncSnapshotSQL = `
SELECT profile_key, payload, sha256, content_sha256, device, app_version, redacted, updated_at
FROM mihosh_sync_snapshots
WHERE profile_key = $1
`

const selectSyncStatusSQL = `
SELECT profile_key, octet_length(payload), sha256, content_sha256, device, app_version, redacted, updated_at
FROM mihosh_sync_snapshots
WHERE profile_key = $1
`

// PostgresStore implements Store backed by PostgreSQL via database/sql + lib/pq.
type PostgresStore struct {
	db *sql.DB
}

// OpenPostgres opens and pings a PostgreSQL connection using the given DSN.
func OpenPostgres(ctx context.Context, dsn string) (Store, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("PostgreSQL DSN is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, postgresConnectionError("open", dsn, err)
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(2 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, postgresConnectionError("connect", dsn, err)
	}
	return &PostgresStore{db: db}, nil
}

func postgresConnectionError(stage, dsn string, cause error) error {
	var parseError *url.Error
	if errors.As(cause, &parseError) {
		// A driver URL error can contain the entire unredacted DSN.
		cause = errors.New("invalid PostgreSQL connection URI")
	}
	return fmt.Errorf("%s postgres (%s): %w", stage, RedactDSN(dsn), cause)
}

func postgresSQLState(err error) string {
	var pgError interface{ SQLState() string }
	if errors.As(err, &pgError) {
		return pgError.SQLState()
	}
	return ""
}

// Push upserts a compressed sync snapshot for snap.Meta.ProfileKey.
func (s *PostgresStore) Push(ctx context.Context, snap Snapshot) error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is closed")
	}
	key := NormalizeProfileKey(snap.Meta.ProfileKey)
	if len(snap.Payload) == 0 {
		return errors.New("sync snapshot payload is empty")
	}
	sum := PayloadSHA256(snap.Payload)
	updatedAt := snap.Meta.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	writeSnapshot := func() error {
		_, err := s.db.ExecContext(ctx, upsertSyncSnapshotSQL, key, snap.Payload, sum,
			snap.Meta.ContentSHA, snap.Meta.Device, snap.Meta.AppVersion, snap.Meta.Redacted, updatedAt)
		return err
	}
	err := writeSnapshot()
	if postgresSQLState(err) == postgresUndefinedTable {
		if _, createErr := s.db.ExecContext(ctx, createSyncTableSQL); createErr != nil {
			return fmt.Errorf("create sync snapshot table: %w", createErr)
		}
		err = writeSnapshot()
	}
	if postgresSQLState(err) == postgresUndefinedColumn {
		if _, migrateErr := s.db.ExecContext(ctx, addSyncContentSHAColumnSQL); migrateErr != nil {
			return fmt.Errorf("migrate sync snapshot table: %w", migrateErr)
		}
		err = writeSnapshot()
	}
	if err != nil {
		return fmt.Errorf("push sync snapshot: %w", err)
	}
	return nil
}

// Pull fetches the latest sync snapshot for profileKey and verifies its payload checksum.
func (s *PostgresStore) Pull(ctx context.Context, profileKey string) (Snapshot, error) {
	if s == nil || s.db == nil {
		return Snapshot{}, errors.New("postgres store is closed")
	}
	key := NormalizeProfileKey(profileKey)
	var snap Snapshot
	err := s.scanSnapshot(ctx, selectSyncSnapshotSQL, key,
		&snap.Meta.ProfileKey,
		&snap.Payload,
		&snap.Meta.SHA256,
		&snap.Meta.ContentSHA,
		&snap.Meta.Device,
		&snap.Meta.AppVersion,
		&snap.Meta.Redacted,
		&snap.Meta.UpdatedAt,
	)
	if errors.Is(err, ErrSnapshotNotFound) {
		return Snapshot{}, ErrSnapshotNotFound
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("pull sync snapshot: %w", err)
	}
	snap.Meta.SizeBytes = len(snap.Payload)
	if snap.Meta.SHA256 != "" && PayloadSHA256(snap.Payload) != snap.Meta.SHA256 {
		return Snapshot{}, ErrIntegrityMismatch
	}
	return snap, nil
}

// Status returns remote metadata for profileKey without transferring the full payload bytes.
func (s *PostgresStore) Status(ctx context.Context, profileKey string) (SnapshotMeta, error) {
	if s == nil || s.db == nil {
		return SnapshotMeta{}, errors.New("postgres store is closed")
	}
	key := NormalizeProfileKey(profileKey)
	var meta SnapshotMeta
	err := s.scanSnapshot(ctx, selectSyncStatusSQL, key,
		&meta.ProfileKey,
		&meta.SizeBytes,
		&meta.SHA256,
		&meta.ContentSHA,
		&meta.Device,
		&meta.AppVersion,
		&meta.Redacted,
		&meta.UpdatedAt,
	)
	if errors.Is(err, ErrSnapshotNotFound) {
		return SnapshotMeta{}, ErrSnapshotNotFound
	}
	if err != nil {
		return SnapshotMeta{}, fmt.Errorf("query sync status: %w", err)
	}
	return meta, nil
}

func (s *PostgresStore) scanSnapshot(ctx context.Context, query, key string, dest ...any) error {
	err := s.db.QueryRowContext(ctx, query, key).Scan(dest...)
	if postgresSQLState(err) == postgresUndefinedColumn {
		// Legacy schemas do not have a content checksum. Read the existing snapshot without DDL.
		legacyQuery := strings.Replace(query, "content_sha256", "''", 1)
		err = s.db.QueryRowContext(ctx, legacyQuery, key).Scan(dest...)
	}
	if errors.Is(err, sql.ErrNoRows) || postgresSQLState(err) == postgresUndefinedTable {
		return ErrSnapshotNotFound
	}
	return err
}

// Close closes the underlying PostgreSQL connection pool.
func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// NormalizeProfileKey trims key and defaults empty keys to "default".
func NormalizeProfileKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "default"
	}
	return key
}

// PayloadSHA256 returns lowercase hex SHA-256 of payload bytes.
func PayloadSHA256(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
