-- Таблица пользователей
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Таблица остатков товаров (inventory)
CREATE TABLE IF NOT EXISTS inventory (
    product_id BIGSERIAL PRIMARY KEY,
    quantity INT NOT NULL DEFAULT 0,
    CONSTRAINT quantity_non_negative CHECK (quantity >= 0)
);

-- Таблица заказов
CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL,
    -- Для микросервисной архитектуры база данных заказов не имеет прямой связи с таблицей inventory,
    -- и может хранить только идентификатор продукта. Логика проверки наличия товара будет обрабатываться через gRPC запросы.
    quantity INT NOT NULL CHECK (quantity > 0),
    status VARCHAR(50) NOT NULL DEFAULT 'created',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);