//go:build generate

package db

import _ "github.com/sqlc-dev/sqlc/cmd/sqlc"

//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc generate
