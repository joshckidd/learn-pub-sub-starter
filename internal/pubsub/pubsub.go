package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int

const (
	Durable SimpleQueueType = iota
	Transient
)

type Acktype int

const (
	Ack Acktype = iota
	NackRequeue
	NackDiscard
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	body, err := json.Marshal(val)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err != nil {
		return err
	}
	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	queueDurable := false
	autoDelete := false
	exclusive := false

	switch queueType {
	case Durable:
		queueDurable = true
	case Transient:
		autoDelete = true
		exclusive = true
	}

	queue, err := ch.QueueDeclare(queueName, queueDurable, autoDelete, exclusive, false, amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	})
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	err = ch.QueueBind(queueName, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	return ch, queue, nil
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
	handler func(T) Acktype,
) error {
	return subscribe(
		conn,
		exchange,
		queueName,
		key,
		queueType,
		handler,
		func(v []byte) (T, error) {
			var res T
			err := json.Unmarshal(v, &res)
			return res, err
		},
	)
}

func SubscribeGOB[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
	handler func(T) Acktype,
) error {
	return subscribe(
		conn,
		exchange,
		queueName,
		key,
		queueType,
		handler,
		func(v []byte) (T, error) {
			buf := bytes.NewBuffer(v)
			dec := gob.NewDecoder(buf)
			var res T
			err := dec.Decode(&res)
			return res, err
		},
	)
}

func PublishGOB[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(val)
	if err != nil {
		return err
	}
	body := buf.Bytes()

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/gob",
		Body:        body,
	})
	if err != nil {
		return err
	}
	return nil
}

func PublishGameLog(ch *amqp.Channel, user, val string) error {
	key := fmt.Sprintf("%s.%s", routing.GameLogSlug, user)

	err := PublishGOB(ch, routing.ExchangePerilTopic, key, routing.GameLog{
		Message:     val,
		Username:    user,
		CurrentTime: time.Now(),
	})
	if err != nil {
		return err
	}
	return nil
}

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) Acktype,
	unmarshaller func([]byte) (T, error),
) error {
	ch, _, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}

	err = ch.Qos(10, 0, false)
	if err != nil {
		return err
	}

	del, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	var res error
	res = nil
	go func() {
		for mess := range del {
			v, err := unmarshaller(mess.Body)
			res = err
			at := handler(v)

			switch at {
			case Ack:
				mess.Ack(false)
			case NackRequeue:
				mess.Nack(false, true)
			case NackDiscard:
				mess.Nack(false, false)
			}

		}
	}()
	return res
}
