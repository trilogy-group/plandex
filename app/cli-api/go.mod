module plandex-cli-api

go 1.23.3

toolchain go1.23.11

require (
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.0
	github.com/rs/cors v1.10.1
)

// Local module references - adjust paths as needed
replace plandex-cli => ../cli

replace plandex-shared => ../shared
