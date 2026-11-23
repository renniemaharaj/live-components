module github.com/renniemaharaj/live-components/examples

go 1.25.3

require (
	github.com/gorilla/websocket v1.5.3
	github.com/renniemaharaj/go-api-kit v0.0.0
	github.com/renniemaharaj/live-components v0.0.0
	golang.org/x/net v0.40.0
)

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/go-chi/chi/v5 v5.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	golang.org/x/time v0.14.0 // indirect
)

replace github.com/renniemaharaj/go-api-kit => ../../go-api-kit

replace github.com/renniemaharaj/live-components => ..
