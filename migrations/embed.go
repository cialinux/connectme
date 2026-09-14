package migrations

import "embed"

//go:embed core/*.sql audit/*.sql identity/*.sql authorization/*.sql locations/*.sql hosts/*.sql credentials/*.sql connections/*.sql
var FS embed.FS
