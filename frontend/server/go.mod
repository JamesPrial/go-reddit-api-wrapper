module github.com/jamesprial/go-reddit-api-wrapper/frontend/server

go 1.25.0

require (
	github.com/golang-jwt/jwt/v5 v5.2.0
	github.com/google/uuid v1.6.0
	github.com/jamesprial/go-reddit-api-wrapper v0.11.2
	golang.org/x/time v0.13.0
)

replace github.com/jamesprial/go-reddit-api-wrapper => ../../
