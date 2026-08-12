//go:build !postgres

package platform

import (
	"database/sql"
	"errors"
)

func OpenPostgres(string) (*sql.DB, error) {
	return nil, errors.New("build with -tags postgres to enable pgx PostgreSQL support")
}
