.PHONY: backend-run backend-test backend-fmt frontend-dev frontend-test frontend-build compose-up compose-down

backend-run:
	cd backend && go run ./cmd/api

backend-test:
	cd backend && go test ./...

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
