module github.com/jamesprial/go-reddit-api-wrapper/frontend/server

go 1.25.0

require (
	github.com/golang-jwt/jwt/v5 v5.2.0
	github.com/google/uuid v1.6.0
	github.com/jamesprial/go-reddit-api-wrapper v0.11.2
	golang.org/x/time v0.13.0
)

require (
	github.com/golang-migrate/migrate/v4 v4.19.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/mattn/go-sqlite3 v1.14.32 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	golang.org/x/sync v0.12.0 // indirect
)

replace github.com/jamesprial/go-reddit-api-wrapper => ../../
