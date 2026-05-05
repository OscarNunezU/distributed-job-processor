package queue

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/logger"
)

const (
	mainQueue = "jobs"
	dlQueue   = "jobs.dead"
	exchange  = "jobs.exchange"
	dlxName   = "jobs.dlx"
)

type RabbitMQConsumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	log     *logger.Logger
}

func NewRabbitMQConsumer(url string, log *logger.Logger) (*RabbitMQConsumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbitmq channel: %w", err)
	}

	if err := ch.Qos(10, 0, false); err != nil {
		return nil, fmt.Errorf("rabbitmq qos: %w", err)
	}

	if err := declareTopology(ch); err != nil {
		return nil, err
	}

	return &RabbitMQConsumer{conn: conn, channel: ch, log: log}, nil
}

func declareTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(dlxName, "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlx: %w", err)
	}

	if _, err := ch.QueueDeclare(dlQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dead-letter queue: %w", err)
	}

	if err := ch.QueueBind(dlQueue, "", dlxName, false, nil); err != nil {
		return fmt.Errorf("bind dlq: %w", err)
	}

	args := amqp.Table{
		"x-dead-letter-exchange": dlxName,
	}
	if _, err := ch.QueueDeclare(mainQueue, true, false, false, false, args); err != nil {
		return fmt.Errorf("declare main queue: %w", err)
	}

	return nil
}

func (c *RabbitMQConsumer) Consume(_ context.Context) (<-chan domain.JobMessage, error) {
	deliveries, err := c.channel.Consume(mainQueue, "", false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq consume: %w", err)
	}

	out := make(chan domain.JobMessage, 64)
	go func() {
		defer close(out)
		for d := range deliveries {
			var job domain.Job
			if err := json.Unmarshal(d.Body, &job); err != nil {
				c.log.Error("failed to unmarshal job", "error", err)
				_ = d.Nack(false, false)
				continue
			}
			out <- domain.JobMessage{
				Job:         &job,
				RawBody:     d.Body,
				DeliveryTag: d.DeliveryTag,
			}
		}
	}()

	return out, nil
}

func (c *RabbitMQConsumer) Ack(_ context.Context, msg domain.JobMessage) error {
	return c.channel.Ack(msg.DeliveryTag, false)
}

func (c *RabbitMQConsumer) Nack(_ context.Context, msg domain.JobMessage, requeue bool) error {
	return c.channel.Nack(msg.DeliveryTag, false, requeue)
}

func (c *RabbitMQConsumer) Close() error {
	if err := c.channel.Close(); err != nil {
		return err
	}
	return c.conn.Close()
}
