.PHONY: backend-run backend-test backend-integration-test backend-fmt frontend-dev frontend-test frontend-build compose-up compose-down migrate-up migrate-down migrate-create db-seed

backend-run:
	cd backend && go run ./cmd/api

backend-test:
	cd backend && go test ./...

backend-integration-test:
	cd backend && go test -p 1 -tags=integration ./...

backend-fmt:
	cd backend && gofmt -w .

frontend-dev:
	npm --prefix frontend run dev

frontend-test:
	npm --prefix frontend test

frontend-build:
	npm --prefix frontend run build

compose-up:
	docker compose --env-file .env up --build

compose-down:
	docker compose --env-file .env down

migrate-up:
	docker compose --env-file .env run --rm migrate up

migrate-down:
	docker compose --env-file .env run --rm migrate down

migrate-create:
	test -n "$(name)"
	cd backend && go run ./cmd/migrate create "$(name)"

db-seed:
	docker compose --env-file .env run --rm db-seed
