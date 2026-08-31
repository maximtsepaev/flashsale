# FlashSale

FlashSale - это высоконагруженная микросервисная платформа для проведения мгновенных распродаж (flash sales). Архитектура проекта спроектирована с учетом требований к высокой пропускной способности, устойчивости к пиковым нагрузкам, защите от состояния гонки (race conditions) и гарантированному исключению повторной продажи дефицитного товара (overselling).

Проект написан на языке Go с использованием чистой слоистой архитектуры и паттерна Repository (Store). Система реализует ключевые паттерны распределенных систем: синхронное взаимодействие сервисов по протоколу gRPC, асинхронную буферизацию заказов через брокер сообщений Apache Kafka, изолированный слой доступа к данным, атомарное резервирование на уровне СУБД PostgreSQL и авторизацию пользователей по стандарту JWT.

---

## Архитектура системы

В моменты проведения распродаж на систему обрушивается шквал одновременных запросов. Классическая монолитная архитектура с синхронной записью в базу данных быстро упирается в блокировки строк и исчерпание пула соединений.

Для решения этой проблемы в FlashSale применен гибридный подход:

1. **Синхронная проверка и резервация остатка**: Проверка наличия и списание товара выполняются максимально быстро и атомарно в сервисе инвентаря через gRPC. Если товара нет, запрос отклоняется на ранней стадии без нагрузки на очередь заказов.
2. **Асинхронное сохранение заказа**: При успешном резервировании шлюз не тратит время на тяжелую транзакцию создания заказа в БД, а мгновенно публикует событие в Apache Kafka и возвращает клиенту успешный ответ (HTTP 201).
3. **Фоновая консистентность (Eventual Consistency)**: Фоновый сервис заказов вычитывает сообщения из Kafka с контролируемой скоростью и фиксирует заказы в базе данных через специализированный слой хранилища.
4. **Слой доступа к данным (Store / Repository)**: Вся логика взаимодействия с PostgreSQL вынесена в изолированные структуры хранилищ (`UserStore`, `InventoryStore`, `OrderStore`) с обязательной сквозной передачей контекста (`context.Context`).

```text
[ HTTP Клиент / Фронтенд ]
            |
            | HTTP (JSON / JWT)
            v
+-------------------------------------------------------+
|                    Gateway Service                    |
|  - Маршрутизация и аутентификация (JWT)               |
|  - Валидация входных данных                           |
|  - UserStore (слой работы с пользователями)           |
+-------------------------------------------------------+
       |                                     |
       | gRPC (Синхронно)                    | Kafka (Асинхронно)
       | ReserveProduct()                    | Topic: "orders"
       v                                     v
+---------------------------+       +---------------------------+
|     Inventory Service     |       |       Apache Kafka        |
|  - gRPC сервер (:50051)   |       |  - Буферизация заказов    |
|  - InventoryStore         |       +---------------------------+
+---------------------------+                     |
       |                                          | Consumer Group:
       | SQL (Атомарный UPDATE)                   | "order-processors"
       v                                          v
+---------------------------+       +---------------------------+
|    PostgreSQL (Storage)   | <---- |       Order Service       |
|  - Таблица inventory      |  SQL  |  - Чтение событий         |
|  - Таблица users          |       |  - OrderStore             |
|  - Таблица orders         |       +---------------------------+
+---------------------------+
```

---

## Основные компоненты и сервисы

Проект разделен на три независимых сервиса и общий слой данных:

### 1. API Gateway (`cmd/gateway`)
Входная точка для внешних клиентов.
- Реализован на HTTP-фреймворке Gin.
- Обеспечивает регистрацию и вход пользователей с выдачей JWT-токенов.
- Хэширование паролей выполнено с использованием криптографической соли и алгоритма SHA-256.
- Содержит промежуточные обработчики (middleware) для структурированного логирования (`slog`) и проверки токенов (`AuthMiddleware`).
- Использует `store.UserStore` для работы с учетными записями пользователей.
- Выступает gRPC-клиентом для сервиса остатков (`InventoryServiceClient`).
- Выступает продюсером сообщений для брокера Apache Kafka (`OrderProducer`).
- Поддерживает Graceful Shutdown для корректного завершения активных соединений при остановке.

### 2. Inventory Service (`cmd/inventory`)
Высокопроизводительный внутренний сервис учета и резервирования складских остатков.
- Предоставляет контракт gRPC, описанный в Protobuf (`api/proto/inventory/inventory.proto`).
- Метод `GetStock`: получение текущего доступного количества товара по его `product_id`.
- Метод `ReserveProduct`: делегирует выполнение атомарного списания в `store.InventoryStore`.
- Запрос выполняет SQL-команду:
  ```sql
  UPDATE inventory 
  SET quantity = quantity - $1 
  WHERE product_id = $2 AND quantity >= $1;
  ```
  Благодаря условию `quantity >= $1` и механизму блокировки строк в PostgreSQL исключаются гонки данных при параллельных запросах и предотвращается отрицательный остаток.
- Слушает внутренний порт TCP `:50051`.

### 3. Order Service (`cmd/order`)
Асинхронный воркер обработки заказов.
- Подключается к Apache Kafka в качестве консьюмера (группа `order-processors`, топик `orders`).
- Непрерывно вычитывает события об успешно зарезервированных товарах.
- Выполняет запись информации о заказе через `store.OrderStore` со статусом `created`.
- Корректно завершает обработку при получении системных сигналов SIGINT/SIGTERM без потери сообщений.

### 4. Слой хранилища данных (Data Store Layer - `internal/store`)
Инкапсулирует SQL-запросы и работу с базой данных:
- **`UserStore` (`internal/store/user.go`)**: методы `CreateUser` (создание пользователя с возвратом ID) и `GetByEmail` (поиск пользователя по email).
- **`InventoryStore` (`internal/store/inventory.go`)**: методы `GetStock` (получение остатка) и `Reserve` (атомарное списание товара с проверкой количества).
- **`OrderStore` (`internal/store/order.go`)**: метод `CreateOrder` (сохранение созданного заказа в базу данных).
- Все методы хранилищ принимают `context.Context` для контроля таймаутов и отмены операций.

---

## Стек технологий

- **Язык разработки**: Go (1.25+)
- **HTTP фреймворк**: Gin (`github.com/gin-gonic/gin`)
- **Межсервисное взаимодействие**: gRPC (`google.golang.org/grpc`), Protocol Buffers v3
- **Брокер сообщений**: Apache Kafka (`github.com/segmentio/kafka-go`) в режиме KRaft
- **СУБД**: PostgreSQL 15
- **Работа с базой данных**: `sqlx` (`github.com/jmoiron/sqlx`), драйвер `pq`
- **Миграции БД**: `golang-migrate`
- **Аутентификация**: JWT (JSON Web Tokens) по стандарту HMAC-SHA256 (`github.com/golang-jwt/jwt/v5`)
- **Логирование**: Стандартный пакет `log/slog` с форматированием в JSON
- **Контейнеризация**: Docker, Docker Compose
- **Инструменты контроля качества**: `go fmt`, `go vet`, `golangci-lint`, `go test`

---

## Структура базы данных

Схема базы данных инициализируется через миграцию `db/migrations/000001_init_schema.up.sql`.

### Таблица `users`
Хранит учетные данные пользователей.
```sql
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### Таблица `inventory`
Хранит доступные остатки товаров на складе. Ограничение `CHECK (quantity >= 0)` гарантирует физическую целостность на уровне СУБД.
```sql
CREATE TABLE IF NOT EXISTS inventory (
    product_id BIGSERIAL PRIMARY KEY,
    quantity INT NOT NULL DEFAULT 0,
    CONSTRAINT quantity_non_negative CHECK (quantity >= 0)
);
```

### Таблица `orders`
Хранит сформированные заказы.
```sql
CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0),
    status VARCHAR(50) NOT NULL DEFAULT 'created',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
```

---

## Структура репозитория

```text
flashsale/
├── api/
│   └── proto/
│       └── inventory/
│           └── inventory.proto       # Protobuf-определение сервиса остатков
├── cmd/
│   ├── gateway/
│   │   └── main.go                   # Точка входа в API Gateway
│   ├── inventory/
│   │   └── main.go                   # Точка входа в gRPC Inventory Service
│   └── order/
│       └── main.go                   # Точка входа в Order Consumer Service
├── db/
│   └── migrations/
│       ├── 000001_init_schema.up.sql   # Накат схемы БД
│       └── 000001_init_schema.down.sql # Откат схемы БД
├── internal/
│   ├── auth/
│   │   └── jwt.go                    # Генерация токенов, хэширование паролей
│   ├── client/
│   │   └── inventory/
│   │       └── client.go             # Инициализация gRPC клиента к Inventory
│   ├── handler/
│   │   ├── auth.go                   # Обработчики /register и /login
│   │   ├── handler.go                # Инициализация роутов Gin
│   │   └── order.go                  # Обработчик /orders
│   ├── kafka/
│   │   └── producer.go               # Продюсер сообщений Kafka
│   ├── middleware/
│   │   └── middleware.go             # Логгер и валидация JWT токенов
│   ├── pb/
│   │   └── inventory/                # Сгенерированный код gRPC и Protobuf
│   │       ├── inventory.pb.go
│   │       └── inventory_grpc.pb.go
│   ├── services/
│   │   └── inventory/
│   │       └── server.go             # Реализация логики gRPC InventoryService
│   └── store/
│       ├── inventory.go              # Слой работы с остатками в БД (InventoryStore)
│       ├── order.go                  # Слой работы с заказами в БД (OrderStore)
│       └── user.go                   # Слой работы с пользователями в БД (UserStore)
├── docker-compose.yml                # Инфраструктура (PostgreSQL, Kafka)
├── Makefile                          # Команды сборки, миграций, проверок и запуска
├── go.mod                            # Зависимости Go
└── go.sum
```

---

## Переменные окружения

Каждый сервис конфигурируется с помощью переменных окружения со следующими значениями по умолчанию:

| Переменная | Сервис | Значение по умолчанию | Описание |
| :--- | :--- | :--- | :--- |
| `DATABASE_URL` | Все сервисы | Обязательная | DSN строка подключения к базе данных PostgreSQL |
| `INVENTORY_GRPC_URL` | Gateway | `localhost:50051` | Адрес gRPC-сервера сервиса остатков |
| `KAFKA_BROKER` | Gateway, Order | `localhost:9092` | Адрес брокера Apache Kafka |

Пример строки подключения к БД:
```text
postgres://postgres:super_secret_password@localhost:5432/flashsale?sslmode=disable
```

---

## Быстрый старт и запуск

### Требования к окружению
- Установленный **Go** версии 1.25 или новее
- Установленный **Docker** и **Docker Compose**
- Утилита **Make** (для выполнения команд Makefile)
- Утилита `golang-migrate` (опционально, накатывать миграции можно через Docker-контейнер)

---

### Шаг 1. Запуск инфраструктуры
Запустите контейнеры с базой данных PostgreSQL и брокером Kafka:

```bash
docker compose up -d postgres kafka
```

Проверить статус запущенных контейнеров:
```bash
docker compose ps
```

---

### Шаг 2. Применение миграций базы данных
Для создания таблиц и индексов выполните команду:

```bash
make migrate-up
```

Если утилита `make` не установлена, выполните накатывание миграций через Docker напрямую:
```bash
docker run --rm -v "$(pwd)/db/migrations:/migrations" --network host migrate/migrate -path=/migrations -database "postgres://postgres:super_secret_password@localhost:5432/flashsale?sslmode=disable" up
```

Для отката последней миграции доступна команда:
```bash
make migrate-down
```

---

### Шаг 3. Наполнение тестовыми данными (Складские остатки)
Перед оформлением заказов необходимо добавить хотя бы один товар в таблицу `inventory`.

Подключитесь к PostgreSQL:
```bash
make psql
```
Или напрямую через docker:
```bash
docker exec -it flashsale_postgres psql -U postgres -d flashsale
```

В консоли базы данных добавьте тестовый товар (например, товар с `product_id = 1` и остатком 50 штук):
```sql
INSERT INTO inventory (product_id, quantity) VALUES (1, 50);
```
Для выхода из psql введите `\q`.

---

### Шаг 4. Запуск сервисов приложения

Для полноценной работы системы откройте три отдельных терминала и запустите сервисы:

**Терминал 1: Сервис остатков (Inventory Service)**
```bash
make run-inventory
```
*Сервис запускает gRPC сервер на порту `:50051`, инициализирует `InventoryStore` и подключается к PostgreSQL.*

**Терминал 2: Сервис обработки заказов (Order Service)**
```bash
make run-order
```
*Сервис запускает Kafka Reader, инициализирует `OrderStore`, подписывается на топик `orders` и ожидает поступления сообщений.*

**Терминал 3: API Gateway**
```bash
make run-gateway
```
*Шлюз инициализирует `UserStore`, `InventoryServiceClient`, `OrderProducer` и поднимает HTTP сервер на порту `:8080`.*

---

### Шаг 5. Проверка качества кода и тестов
Для автоматического форматирования, статического анализа и запуска тестов выполните:

```bash
make check
```
Команда последовательно выполняет:
1. `go fmt ./...` - форматирование исходного кода.
2. `go vet ./...` - поиск подозрительных конструкций.
3. `golangci-lint run` - запуск комплексного линтера.
4. `go test ./...` - запуск модульных и интеграционных тестов.

---

## Документация API

Базовый URL шлюза: `http://localhost:8080`

### 1. Регистрация пользователя
Создает нового пользователя в системе через `UserStore`.

- **URL**: `/register`
- **Метод**: `POST`
- **Заголовки**: `Content-Type: application/json`
- **Тело запроса**:
```json
{
  "email": "user@example.com",
  "password": "secretpassword"
}
```

- **Пример cURL**:
```bash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{"email": "buyer@test.com", "password": "mypassword123"}'
```

- **Успешный ответ (201 Created)**:
```json
{
  "message": "User created successfully",
  "user_id": 1
}
```

- **Ошибки**:
  - `400 Bad Request` - некорректный email или длина пароля менее 6 символов.
  - `409 Conflict` - пользователь с таким email уже зарегистрирован или произошла ошибка базы данных.

---

### 2. Аутентификация (Вход)
Проверяет учетные данные через `UserStore` и возвращает JWT токен.

- **URL**: `/login`
- **Метод**: `POST`
- **Заголовки**: `Content-Type: application/json`
- **Тело запроса**:
```json
{
  "email": "buyer@test.com",
  "password": "mypassword123"
}
```

- **Пример cURL**:
```bash
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email": "buyer@test.com", "password": "mypassword123"}'
```

- **Успешный ответ (200 OK)**:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxLCJleHAiOjE3Mzg0MjM4MDAsImlhdCI6MTczODMzNzQwMH0.signature"
}
```

- **Ошибки**:
  - `400 Bad Request` - не переданы обязательные поля.
  - `401 Unauthorized` - неверный email или пароль.

---

### 3. Оформление заказа (Flash Sale Order)
Атомарно списывает товар со склада через `InventoryStore` и отправляет заказ в очередь на асинхронное сохранение.

- **URL**: `/orders`
- **Метод**: `POST`
- **Заголовки**: 
  - `Content-Type: application/json`
  - `Authorization: Bearer <ВАШ_JWT_ТОКЕН>`
- **Тело запроса**:
```json
{
  "product_id": 1,
  "quantity": 2
}
```

- **Пример cURL**:
```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -d '{"product_id": 1, "quantity": 2}'
```

- **Успешный ответ (201 Created)**:
```json
{
  "message": "Order created successfully"
}
```

- **Ошибки**:
  - `401 Unauthorized` - отсутствует или невалиден заголовок `Authorization`.
  - `400 Bad Request` - недостаточно остатка на складе (`"error": "Failed to reserve product", "message": "Not enough stock"`).
  - `500 Internal Server Error` - ошибка взаимодействия с gRPC-сервисом или Kafka.

---

## Проверка сценариев работы

### Сценарий 1: Успешная покупка
1. Добавьте в базу товар с остатком 5 штук.
2. Авторизуйтесь и отправьте заказ на 3 единицы товара.
3. Ответ: HTTP 201 Created.
4. Проверьте логи в терминале Order Service: появится запись о вычитке сообщения из Kafka и сохранении в БД через `OrderStore`.
5. Проверьте остаток в таблице `inventory`: он уменьшился до 2.
6. Проверьте таблицу `orders`: появилась новая запись с `user_id`, `product_id = 1`, `quantity = 3`, `status = 'created'`.

### Сценарий 2: Недостаточно товара (Защита от overselling)
1. При текущем остатке 2 единицы отправьте запрос на покупку 5 единиц.
2. Ответ: HTTP 400 Bad Request с сообщением `"Not enough stock"`.
3. Сообщение в Kafka не отправляется, лишней нагрузки на базу данных заказов не создается, остаток товара в таблице `inventory` не изменился.

---

## Генерация Protobuf (При изменении контракта)

В случае изменения файла `api/proto/inventory/inventory.proto` выполните генерацию Go-кода с помощью компилятора `protoc`:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       internal/pb/inventory/inventory.proto
```

Требуются установленные плагины:
- `google.golang.org/protobuf/cmd/protoc-gen-go`
- `google.golang.org/grpc/cmd/protoc-gen-go-grpc`

---

## Потенциал развития и масштабирования

В текущей архитектуре заложен надежный фундамент для дальнейших оптимизаций:
- **Кэширование остатков в Redis**: предварительное декрементирование счетчиков в Redis (Lua-скрипты) для снятия части нагрузки с PostgreSQL при экстремальном трафике.
- **Паттерн Transactional Outbox**: гарантированная доставка событий в Kafka при сложных распределенных транзакциях.
- **Паттерн Saga / Компенсирующие транзакции**: автоматический возврат зарезервированного товара при сбое оплаты или отмене заказа.
- **Метрики и трассировка**: подключение Prometheus для сбора метрик (RPS, latency, ошибки gRPC) и OpenTelemetry/Jaeger для распределенной трассировки запросов.
- **Rate Limiting**: ограничение частоты запросов от одного IP / UserID для защиты от ботов на шлюзе.
