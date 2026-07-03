package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
)

type OrderMessage struct {
	UserID    int64 `json:"user_id"`
	ProductID int64 `json:"product_id"`
	Quantity  int32 `json:"quantity"`
}

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(broker string, topic string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(broker),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

// PublishOrder отправляет сообщение о заказе в топик Кафки
func (p *Producer) PublishOrder(ctx context.Context, msg OrderMessage) error {
	bytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal order message: %w", err)
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Value: bytes,
	})
	if err != nil {
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}

	return nil
}

// Close закрывает соединение с брокером при остановке сервиса
func (p *Producer) Close() error {
	return p.writer.Close()
}
