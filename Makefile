ifneq (,$(wildcard .env))
    include .env
    export
endif

# Адрес БД для утилиты миграций (запускается внутри Docker контейнера)
DB_URL ?= postgres://postgres:super_secret_password@host.docker.internal:5432/flashsale?sslmode=disable

# Адреса для локального запуска сервисов на хосте (через go run)
DB_LOCAL ?= postgres://postgres:super_secret_password@localhost:5432/flashsale?sslmode=disable
KAFKA_LOCAL ?= localhost:9092
GRPC_LOCAL ?= localhost:50051

.PHONY: up down restart logs build run-inventory run-gateway run-order migrate-up migrate-down psql check proto

# Запуск всех сервисов и инфраструктуры в Docker
up:
	docker compose up -d --build

# Остановка всех контейнеров
down:
	docker compose down

# Перезапуск контейнеров
restart: down up

# Просмотр логов всех сервисов в реальном времени
logs:
	docker compose logs -f

# Сборка образов Docker
build:
	docker compose build

# ---
# ЛОКАЛЬНЫЙ ЗАПУСК СЕРВИСОВ (Для разработки и отладки через IDE / go run)
# Перед запуском поднимите только БД и Кафку: docker compose up -d postgres kafka

# Запуск сервиса остатков (gRPC :50051)
run-inventory:
	DATABASE_URL="$(DB_LOCAL)" GRPC_PORT=50051 go run cmd/inventory/main.go

# Запуск API шлюза (HTTP :8080)
run-gateway:
	DATABASE_URL="$(DB_LOCAL)" KAFKA_BROKER="$(KAFKA_LOCAL)" INVENTORY_GRPC_URL="$(GRPC_LOCAL)" HTTP_PORT=8080 go run cmd/gateway/main.go

# Запуск сервиса обработки заказов (Kafka Consumer)
run-order:
	DATABASE_URL="$(DB_LOCAL)" KAFKA_BROKER="$(KAFKA_LOCAL)" go run cmd/order/main.go
#---

# Накат миграций базы данных
migrate-up:
	MSYS_NO_PATHCONV=1 docker run --rm -v "$$(pwd)/db/migrations:/migrations" migrate/migrate -path=/migrations -database "$(DB_URL)" up

# Откат миграций на 1 шаг назад
migrate-down:
	MSYS_NO_PATHCONV=1 docker run --rm -v "$$(pwd)/db/migrations:/migrations" migrate/migrate -path=/migrations -database "$(DB_URL)" down 1

# Выполнение sql команд в контейнере PostgreSQL
psql:
	docker exec -it flashsale_postgres psql -U postgres -d flashsale
# Использование: В терминале написать make psql, открывается приглашение flashsale=#, вводишь запросы. 
# Для выхода Ctrl+D или \q. 

# Проверка качества кода (форматирование, vet, линтер, тесты)
check:
	go fmt ./...
	go vet ./...
	golangci-lint run
	go test ./...

# Генерация gRPC и Protobuf кода при изменении контракта
proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative api/proto/inventory/inventory.proto