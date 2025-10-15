.PHONY: deps sqlc-gen migrate run build clean

deps:
	go mod tidy

sqlc-gen:
	sqlc generate

migrate:
	goose -dir migrations sqlite3 data/db.sqlite3 up

run:
	go run main.go

build:
	go build -o bin/shotr main.go

clean:
	rm -rf bin data/db.sqlite3