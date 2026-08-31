DB_URL=postgres://postgres:super_secret_password@host.docker.internal:5432/flashsale?sslmode=disable
DB_LOCAL=postgres://postgres:super_secret_password@localhost:5432/flashsale?sslmode=disable
KAFKA_LOCAL=localhost:9092

# Команда для наката миграций
migrate-up:
	MSYS_NO_PATHCONV=1 docker run --rm -v "$$(pwd)/db/migrations:/migrations" migrate/migrate -path=/migrations -database "$(DB_URL)" up

# Команда для отката миграций на 1 шаг назад
migrate-down:
	MSYS_NO_PATHCONV=1 docker run --rm -v "$$(pwd)/db/migrations:/migrations" migrate/migrate -path=/migrations -database "$(DB_URL)" down 1

# Запуск сервиса остатков
run-inventory:
	DATABASE_URL="$(DB_LOCAL)" go run cmd/inventory/main.go

# Запуск шлюза
run-gateway:
	DATABASE_URL="$(DB_LOCAL)" KAFKA_BROKER="$(KAFKA_LOCAL)" go run cmd/gateway/main.go

# Запуск сервиса заказов
run-order:
	DATABASE_URL="$(DB_LOCAL)" KAFKA_BROKER="$(KAFKA_LOCAL)" go run cmd/order/main.go

# Выполнение sql команд в контейнере PostgreSQL
psql:
	docker exec -it flashsale_postgres psql -U postgres -d flashsale
# Использование: В терминале написать make psql, открывается приглашение flashsale=#, вводишь запросы. 
# Для выхода Ctrl+D или \q. 

# Проверка кода на форматирование, ошибки и тесты
check:
    go fmt ./...
    go vet ./...
    golangci-lint run
    go test ./...