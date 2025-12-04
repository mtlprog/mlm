default: build

build:
	sqlc generate
	# go generate ./doc.go

run: build
	go run ./cmd/mlmc/...

test: build
	go test -v -count=1 ./...

migrate-status:
	goose -dir migrations postgres "dbname=mlm sslmode=disable" status

migrate-generate:
	goose -dir migrations postgres "dbname=mlm sslmode=disable" create ${name} sql

migrate-up:
	goose -dir migrations postgres "dbname=mlm sslmode=disable" up
