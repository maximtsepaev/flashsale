# Переменные
DB_URL=postgres://postgres:super_secret_password@localhost:5432/flashsale?sslmode=disable

.PHONY: migrate-up migrate-down

# Применить миграции
migrate-up:
	docker run --rm -v $(CURDIR)/db/migrations:/migrations migrate/migrate -path=/migrations -database "$(DB_URL)" up

# Откатить миграции на 1 шаг назад
migrate-down:
	docker run --rm -v $(CURDIR)/db/migrations:/migrations migrate/migrate -path=/migrations -database "$(DB_URL)" down 1