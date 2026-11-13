// Package migrations embeds the numbered .sql files into the binary,
// so a fresh `docker compose up` creates the schema with no extra tooling.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
