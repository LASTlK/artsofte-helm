.DEFAULT_GOAL := up

.PHONY: up db run check logs down

up:
	docker compose up --build --wait app
	@echo "Application: http://localhost:8080"

db:
	docker compose up --wait db

# Start PostgreSQL in Docker and run Go locally (requires Go).
run: db
	DB_HOST=127.0.0.1 DB_PORT=55432 DB_NAME=counter DB_USER=counter DB_PASSWORD=counter-local-password DB_SSLMODE=disable HTTP_ADDR=:8080 go run .

check:
	curl --fail --silent --show-error http://localhost:8080/healthz
	curl --fail --silent --show-error http://localhost:8080/readyz
	curl --fail --silent --show-error http://localhost:8080/

logs:
	docker compose logs --follow app db

# Keep the database volume and its visit counter.
down:
	docker compose down
