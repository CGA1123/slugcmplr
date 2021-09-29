// +build tools

package tools

import (
	_ "github.com/golang-migrate/migrate/v4/cmd/migrate"
	_ "github.com/kyleconroy/sqlc/cmd/sqlc"
	_ "github.com/twitchtv/twirp/protoc-gen-twirp"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
