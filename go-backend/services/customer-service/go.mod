module stripe-demo/services/customer-service

go 1.23.0

toolchain go1.24.3

require (
	github.com/go-playground/validator/v10 v10.26.0
	github.com/gorilla/mux v1.8.1
	github.com/stripe-ecosystem/shared/contracts v0.0.0
	github.com/stripe-ecosystem/shared/stripe-client v0.0.0
	golang.org/x/time v0.11.0
	github.com/stripe-ecosystem/shared/middleware v0.0.0
)

require (
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/stripe/stripe-go/v82 v82.2.0 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
)

replace github.com/stripe-ecosystem/shared/contracts => ../../shared/contracts

replace github.com/stripe-ecosystem/shared/stripe-client => ../../shared/stripe-client

replace github.com/stripe-ecosystem/shared/middleware => ../../shared/middleware
