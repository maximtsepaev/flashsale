# FlashSale

FlashSale - это высоконагруженная микросервисная платформа для проведения мгновенных распродаж (flash sales). Архитектура проекта спроектирована с учетом требований к высокой пропускной способности, устойчивости к пиковым нагрузкам, защите от состояния гонки (race conditions) и гарантированному исключению повторной продажи дефицитного товара (overselling).

Проект написан на языке Go (1.25) с использованием чистой слоистой архитектуры и паттерна Repository (Store). Система реализует ключевые паттерны распределенных систем: синхронное взаимодействие сервисов по протоколу gRPC, асинхронную буферизацию заказов через брокер сообщений Apache Kafka, изолированный слой доступа к данным, атомарное резервирование на уровне СУБД PostgreSQL, авторизацию пользователей по стандарту JWT и полную контейнеризацию в Docker Compose.

---

## Архитектура системы

В моменты проведения распродаж на систему обрушивается пиковый поток одновременных запросов. Классическая монолитная архитектура с синхронной записью в базу данных быстро упирается в блокировки строк и исчерпание пула соединений.

Для решения этой проблемы в FlashSale применен гибридный подход:

1. **Синхронная проверка и резервация остатка**: Проверка наличия и списание товара выполняются максимально быстро и атомарно в сервисе инвентаря через gRPC. Если товара нет, запрос отклоняется на ранней стадии без нагрузки на очередь заказов.
2. **Асинхронное сохранение заказа**: При успешном резервировании шлюз не тратит время на тяжелую транзакцию создания заказа в БД, а мгновенно публикует событие в Apache Kafka и возвращает клиенту успешный ответ (HTTP 201).
3. **Фоновая консистентность (Eventual Consistency)**: Фоновый сервис заказов вычитывает сообщения из Kafka с контролируемой скоростью и фиксирует заказы в базе данных через специализированный слой хранилища.
4. **Слой доступа к данным (Store / Repository)**: Вся логика взаимодействия с PostgreSQL вынесена в изолированные структуры хранилищ (`UserStore`, `InventoryStore`, `OrderStore`) с обязательной сквозной передачей контекста (`context.Context`).

<img width="1040" height="960" alt="My First Board (1)" src="https://github.com/user-attachments/assets/2b63702f-1637-45ae-b52c-b5ac29ef95c2" />

---

## Основные компоненты и сервисы

Проект разделен на три независимых сервиса и общий слой данных:

### 1. API Gateway (`cmd/gateway`)
Входная точка для внешних клиентов:
- Реализован на HTTP-фреймворке Gin.
- Порт конфигурируется через переменную окружения `HTTP_PORT` (по умолчанию `:8080`).
- Обеспечивает регистрацию и вход пользователей с выдачей JWT-токенов.
- Секретный ключ подписи JWT считывается из переменной окружения `JWT_SECRET`.
- Хэширование паролей выполнено с использованием криптографической соли и алгоритма SHA-256.
- Содержит промежуточные обработчики (middleware) для структурированного логирования (`slog`) и проверки токенов (`AuthMiddleware`).
- Использует `store.UserStore` для работы с учетными записями пользователей.
- Выступает gRPC-клиентом для сервиса остатков (`InventoryServiceClient`).
- Выступает продюсером сообщений для брокера Apache Kafka (`OrderProducer`).
- Поддерживает Graceful Shutdown для корректного завершения активных соединений при остановке.

### 2. Inventory Service (`cmd/inventory`)
Высокопроизводительный внутренний сервис учета и резервирования складских остатков:
- Порт gRPC настраивается через переменную `GRPC_PORT` (по умолчанию `:50051`).
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

### 3. Order Service (`cmd/order`)
Асинхронный воркер обработки заказов:
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

- **Язык разработки**: Go 1.25
- **HTTP фреймворк**: Gin (`github.com/gin-gonic/gin`)
- **Межсервисное взаимодействие**: gRPC (`google.golang.org/grpc`), Protocol Buffers v3
- **Брокер сообщений**: Apache Kafka (`github.com/segmentio/kafka-go`) в режиме KRaft
- **СУБД**: PostgreSQL 15
- **Работа с базой данных**: `sqlx` (`github.com/jmoiron/sqlx`), драйвер `pq`
- **Миграции БД**: `golang-migrate` (через Docker-контейнер)
- **Аутентификация**: JWT (JSON Web Tokens) по стандарту HMAC-SHA256 (`github.com/golang-jwt/jwt/v5`)
- **Логирование**: Стандартный пакет `log/slog` с форматированием в JSON
- **Контейнеризация**: Docker, Docker Compose (Multi-stage сборка образов на базе `golang:1.25-alpine` и `alpine:latest`)
- **Инструменты контроля качества**: `go fmt`, `go vet`, `golangci-lint`, `go test`

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
│   │   ├── Dockerfile                # Multi-stage Dockerfile для API Gateway
│   │   └── main.go                   # Точка входа в API Gateway
│   ├── inventory/
│   │   ├── Dockerfile                # Multi-stage Dockerfile для Inventory Service
│   │   └── main.go                   # Точка входа в gRPC Inventory Service
│   └── order/
│       ├── Dockerfile                # Multi-stage Dockerfile для Order Service
│       └── main.go                   # Точка входа в Order Consumer Service
├── db/
│   └── migrations/
│       ├── 000001_init_schema.up.sql   # Накат схемы БД
│       └── 000001_init_schema.down.sql # Откат схемы БД
├── internal/
│   ├── auth/
│   │   └── jwt.go                    # Генерация токенов, валидация, хэширование паролей
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
├── .env.example                      # Шаблон конфигурации переменных окружения
├── .env                              # Локальный файл конфигурации (в .gitignore)
├── .gitignore                        # Исключения для Git
├── docker-compose.yml                # Запуск всех сервисов (PostgreSQL, Kafka, Services)
├── Makefile                          # Команды сборки, запуска, миграций и проверок
├── go.mod                            # Зависимости Go (Go 1.25.5)
└── go.sum
```

---

## Переменные окружения (`.env`)

Все параметры проекта настраиваются через файл `.env`. Для быстрого старта скопируйте шаблон:

```bash
cp .env.example .env
```

| Переменная | Сервисы | Значение по умолчанию | Описание |
| :--- | :--- | :--- | :--- |
| `POSTGRES_USER` | PostgreSQL, Compose | `postgres` | Пользователь базы данных |
| `POSTGRES_PASSWORD`| PostgreSQL, Compose | `super_secret_password` | Пароль базы данных |
| `POSTGRES_DB` | PostgreSQL, Compose | `flashsale` | Название базы данных |
| `POSTGRES_PORT` | PostgreSQL, Compose | `5432` | Внешний порт PostgreSQL |
| `DATABASE_URL` | Все сервисы | Обязательная | DSN строка подключения к PostgreSQL |
| `KAFKA_PORT` | Kafka, Compose | `9092` | Внешний порт Kafka для хоста |
| `KAFKA_BROKER` | Gateway, Order | `localhost:9092` / `kafka:29092` | Адрес брокера Apache Kafka |
| `HTTP_PORT` | Gateway | `8080` | Порт HTTP сервера API Gateway |
| `JWT_SECRET` | Gateway (Auth) | Обязательная | Секретный ключ подписи JWT-токенов |
| `GRPC_PORT` | Inventory | `50051` | Порт gRPC сервера сервиса остатков |
| `INVENTORY_GRPC_URL`| Gateway | `localhost:50051` / `inventory:50051` | Адрес gRPC сервиса остатков |

---

## Быстрый старт и запуск

### Требования к окружению
- Установленный **Go** версии 1.25+ (для локального запуска вне Docker)
- Установленный **Docker** и **Docker Compose**
- Утилита **Make**

---

### Вариант 1. Запуск всего проекта в Docker Compose (Рекомендуемый)

Запуск всех сервисов (`postgres`, `kafka`, `inventory`, `gateway`, `order`) в один клик:

```bash
# 1. Запустить все контейнеры со сборкой
make up

# 2. Накатить миграции базы данных
make migrate-up

# 3. Добавить тестовый товар на склад
make psql
# В консоли Postgres выполнить:
# INSERT INTO inventory (product_id, quantity) VALUES (1, 50);
# \q для выхода

# 4. Просмотр логов в реальном времени
make logs
```

Проверить статус запущенных контейнеров:
```bash
docker compose ps
```

Остановить все сервисы:
```bash
make down
```

---

### Вариант 2. Локальная разработка и отладка

Используется, когда нужно быстро писать код и отлаживать сервисы в IDE без пересборки Docker-образов:

**Шаг 1. Запуск только базы данных и Kafka в Docker:**
```bash
docker compose up -d postgres kafka
```

**Шаг 2. Применение миграций БД:**
```bash
make migrate-up
```

**Шаг 3. Добавление тестового товара:**
```bash
make psql
# INSERT INTO inventory (product_id, quantity) VALUES (1, 50);
# \q
```

**Шаг 4. Запуск сервисов в трех отдельных терминалах:**

* **Терминал 1 (Inventory Service - gRPC :50051):**
  ```bash
  make run-inventory
  ```
* **Терминал 2 (Order Service - Kafka Consumer):**
  ```bash
  make run-order
  ```
* **Терминал 3 (API Gateway - HTTP :8080):**
  ```bash
  make run-gateway
  ```

---

## Справочник команд Makefile

| Команда | Описание |
| :--- | :--- |
| `make up` | Собрать образы и запустить все 5 сервисов в Docker |
| `make down` | Остановить и удалить все контейнеры проекта |
| `make restart` | Полный перезапуск контейнеров (`down` + `up`) |
| `make logs` | Просмотр объединенных логов всех сервисов в реальном времени |
| `make build` | Сборка Docker-образов без запуска |
| `make migrate-up` | Применение всех миграций схемы базы данных |
| `make migrate-down` | Откат последней миграции базы данных на 1 шаг назад |
| `make psql` | Интерактивное подключение к PostgreSQL через консоль `psql` |
| `make run-inventory` | Локальный запуск сервиса остатков на хосте (`:50051`) |
| `make run-gateway` | Локальный запуск шлюза на хосте (`:8080`) |
| `make run-order` | Локальный запуск сервиса заказов на хосте |
| `make check` | Запуск форматирования (`go fmt`), анализатора (`go vet`), линтера (`golangci-lint`) и тестов |
| `make proto` | Генерация Go-кода Protobuf и gRPC из файла `inventory.proto` |

---

## Структура базы данных

Схема базы данных инициализируется через миграцию `db/migrations/000001_init_schema.up.sql`.

### Таблица `users`
```sql
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### Таблица `inventory`
```sql
CREATE TABLE IF NOT EXISTS inventory (
    product_id BIGSERIAL PRIMARY KEY,
    quantity INT NOT NULL DEFAULT 0,
    CONSTRAINT quantity_non_negative CHECK (quantity >= 0)
);
```

### Таблица `orders`
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

## Документация API

Базовый URL шлюза: `http://localhost:8080`

### 1. Регистрация пользователя
Создает нового пользователя в системе.

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
Проверяет учетные данные и возвращает JWT токен.

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
Атомарно списывает товар со склада через `InventoryService` и отправляет заказ в Kafka для асинхронного сохранения в БД.

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
  - `401 Unauthorized` — отсутствует или невалиден заголовок `Authorization`.
  - `400 Bad Request` — недостаточно товара на складе (`"message": "Not enough stock"`).
  - `500 Internal Server Error` — инфраструктурная ошибка связи с gRPC или Kafka.

---

## Проверка сценариев работы

### Сценарий 1: Успешная покупка
1. Добавьте в базу товар с остатком 5 штук (`INSERT INTO inventory (product_id, quantity) VALUES (1, 5);`).
2. Авторизуйтесь через `/login` и получите JWT-токен.
3. Отправьте запрос на покупку 3 единиц товара через `POST /orders`.
4. Ответ: `HTTP 201 Created`.
5. Проверьте логи сервиса заказов: `make logs` покажет вычитку сообщения из Kafka и сохранение в PostgreSQL.
6. Проверьте остаток в таблице `inventory`: он уменьшился до 2.
7. Проверьте таблицу `orders`: появилась запись со статусом `created`.

### Сценарий 2: Недостаточно товара (Защита от overselling)
1. При текущем остатке 2 единицы отправьте запрос на покупку 5 единиц.
2. Ответ: `HTTP 400 Bad Request` с сообщением `"Not enough stock"`.
3. Сообщение в Kafka не отправляется, лишней нагрузки на базу данных заказов не создается, остаток товара в таблице `inventory` не изменился.

---

## Генерация Protobuf (При изменении контракта)

При изменении файла `api/proto/inventory/inventory.proto` запустите команду:

```bash
make proto
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

