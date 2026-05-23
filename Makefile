postgres://flashsale:super_seccret_flash_sale_password@localhost:5432/flashsale?sslmode=disable# Переменные
DB_URL=

# Применение миграций
migrate-up:
	docker run --rm -v $(CURDIR)/db/migrations:/migrations migrate/migrate -path=/migrations -database "$(DB_URL)" up

# Откат миграций
migrate-down:
	docker run --rm -v $(CURDIR)/db/migrations:/migrations migrate/migrate -path=/migrations -database "$(DB_URL)" down 1