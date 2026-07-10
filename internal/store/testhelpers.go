//go:build testonly

package store

import "database/sql"

// RawDB returns the underlying *sql.DB so integration tests can run
// raw queries for detailed verification of persisted data.
func (s *Store) RawDB() *sql.DB {
	return s.db
}
