package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type connectionAPI interface {
	Channel() (channelAPI, error)
	Close() error
}

type channelAPI interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Close() error
}

type amqpConnection struct {
	conn *amqp.Connection
}

func (c *amqpConnection) Channel() (channelAPI, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}
	return &amqpChannel{channel: ch}, nil
}

func (c *amqpConnection) Close() error {
	return c.conn.Close()
}

type amqpChannel struct {
	channel *amqp.Channel
}

func (c *amqpChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	return c.channel.ExchangeDeclare(name, kind, durable, autoDelete, internal, noWait, args)
}

func (c *amqpChannel) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	return c.channel.PublishWithContext(ctx, exchange, key, mandatory, immediate, msg)
}

func (c *amqpChannel) Close() error {
	return c.channel.Close()
}

var dialAMQP = func(url string) (connectionAPI, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	return &amqpConnection{conn: conn}, nil
}

type Publisher struct {
	conn    connectionAPI
	channel channelAPI
}

type MessagePublisher interface {
	Publish(routingKey string, payload interface{}) error
}

func NewPublisher(url string) (*Publisher, error) {
	conn, err := dialAMQP(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	err = ch.ExchangeDeclare(
		"video_events", // name
		"topic",        // type
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare an exchange: %w", err)
	}

	return &Publisher{
		conn:    conn,
		channel: ch,
	}, nil
}

func (p *Publisher) Publish(routingKey string, payload interface{}) error {
	if p == nil || p.channel == nil || p.conn == nil {
		return fmt.Errorf("publisher not initialized")
	}

	if routingKey == "" {
		return fmt.Errorf("routing key cannot be empty")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	err = p.channel.PublishWithContext(
		context.Background(),
		"video_events",
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

func (p *Publisher) Close() {
	if p == nil {
		return
	}

	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}
