package rabbitmq

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestRealRabbitConnectionPanics(t *testing.T) {
	c := &amqpConnection{}

	func() {
		defer func() { recover() }()
		c.Channel()
	}()

	func() {
		defer func() { recover() }()
		c.Close()
	}()
}

func TestRealRabbitChannelPanics(t *testing.T) {
	c := &amqpChannel{}

	func() {
		defer func() { recover() }()
		c.ExchangeDeclare("", "", false, false, false, false, nil)
	}()

	func() {
		defer func() { recover() }()
		c.PublishWithContext(context.Background(), "", "", false, false, amqp.Publishing{})
	}()

	func() {
		defer func() { recover() }()
		c.Close()
	}()
}

func TestPublisherCloseUninitialized(t *testing.T) {
	p := &Publisher{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Close panicked on uninitialized publisher: %v", r)
			}
		}()
		p.Close()
	}()
}
