package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"runtime"
)

type OpenOptions struct {
	CacheSize int
	Params    map[string]string
}

func OpenReadWrite(ctx context.Context, dbFile string, opts OpenOptions) (rdb *sql.DB, wdb *sql.DB, err error) {
	uri := &url.URL{
		Scheme: "file",
		Opaque: dbFile,
	}
	query := uri.Query()
	if opts.Params != nil {
		for k, v := range opts.Params {
			query.Set(k, v)
		}
	}
	query.Set("_txlock", "immediate")
	uri.RawQuery = query.Encode()

	pragmas := fmt.Sprintf(`
		PRAGMA journal_mode = WAL;
		PRAGMA busy_timeout = 5000;
		PRAGMA synchronous = NORMAL;
		PRAGMA cache_size = -%d;
		PRAGMA foreign_keys = true;
		PRAGMA temp_store = memory;
	`, opts.CacheSize)

	readConn, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			readConn.Close()
		}
	}()
	readConn.SetMaxOpenConns(max(4, runtime.NumCPU()))

	if _, err = readConn.ExecContext(ctx, pragmas); err != nil {
		return nil, nil, fmt.Errorf("configure db conn: %w", err)
	}

	writeConn, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			writeConn.Close()
		}
	}()
	writeConn.SetMaxOpenConns(1)

	if _, err = writeConn.ExecContext(ctx, pragmas); err != nil {
		return nil, nil, fmt.Errorf("configure db conn: %w", err)
	}

	if _, err = writeConn.ExecContext(ctx, Schema); err != nil {
		return nil, nil, fmt.Errorf("init db: %w", err)
	}

	return readConn, writeConn, nil
}
