package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"runtime"

	"github.com/mattn/go-sqlite3"
)

type OpenOptions struct {
	Params map[string]string
}

func RegisterPragmaHook(cacheSize int) {
	sql.Register("toxstatus_sqlite3", &sqlite3.SQLiteDriver{
		ConnectHook: func(c *sqlite3.SQLiteConn) error {
			fmt.Println("Executing pragmas")
			pragmas := fmt.Sprintf(`
				PRAGMA journal_mode = WAL;
				PRAGMA busy_timeout = 5000;
				PRAGMA synchronous = NORMAL;
				PRAGMA cache_size = -%d;
				PRAGMA foreign_keys = true;
				PRAGMA temp_store = memory;
			`, cacheSize)
			_, err := c.Exec(pragmas, nil)
			return err
		},
	})
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

	readConn, err := sql.Open("toxstatus_sqlite3", uri.String())
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			readConn.Close()
		}
	}()
	readConn.SetMaxOpenConns(max(4, runtime.NumCPU()))

	writeConn, err := sql.Open("toxstatus_sqlite3", uri.String())
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			writeConn.Close()
		}
	}()
	writeConn.SetMaxOpenConns(1)

	if _, err = writeConn.ExecContext(ctx, Schema); err != nil {
		return nil, nil, fmt.Errorf("init db: %w", err)
	}

	return readConn, writeConn, nil
}
