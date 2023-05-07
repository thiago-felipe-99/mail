package rabbit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Message = amqp.Delivery

var (
	ErrAlreadyClosed    = errors.New("connection already closed")
	ErrConnectionClosed = errors.New("closed connection with RabbitMQ")
	ErrEncondingMessage = errors.New("error encoding message")
	ErrSendingMessage   = errors.New("error sending message")
	ErrTimeoutMessage   = errors.New("timeout sending message")
	ErrMaxRetries       = errors.New("error max retries")
)

type MaxRetriesError struct {
	errors []error
}

func (maxRetriesError *MaxRetriesError) add(err error) {
	maxRetriesError.errors = append(maxRetriesError.errors, err)
}

func (maxRetriesError *MaxRetriesError) Error() string {
	maxRetriesError.errors = append([]error{ErrMaxRetries}, maxRetriesError.errors...)

	return errors.Join(maxRetriesError.errors...).Error()
}

type Config struct {
	User     string
	Password string
	Host     string
	Port     string
	Vhost    string
}

type Rabbit struct {
	url                string
	maxRetries         int
	timeoutSendMessage time.Duration

	close      bool
	connection *amqp.Connection
}

func (rabbit *Rabbit) Close() error {
	if rabbit.close {
		return ErrAlreadyClosed
	}

	rabbit.close = true

	err := rabbit.connection.Close()
	if err != nil {
		return fmt.Errorf("error on closing RabbitMQ connection: %w", err)
	}

	return nil
}

func (rabbit *Rabbit) retries(maxRetries int, errsReturn []error, try func() error) error {
	resendDelay := time.Second
	errMaxRetries := &MaxRetriesError{}

	for retries := 0; retries < maxRetries; retries++ {
		err := try()
		if err != nil {
			for _, errReturn := range errsReturn {
				if errors.Is(err, errReturn) {
					return err
				}
			}

			errMaxRetries.add(err)

			time.Sleep(resendDelay)

			resendDelay *= 2

			continue
		}

		return nil
	}

	return errMaxRetries
}

func (rabbit *Rabbit) CreateQueueWithDLX(name string, dlx string, maxRetries int64) error {
	errsReturn := []error{}

	createQueue := func() error {
		return rabbit.createQueueWithDLX(name, dlx, maxRetries)
	}

	return rabbit.retries(rabbit.maxRetries, errsReturn, createQueue)
}

func (rabbit *Rabbit) createQueueWithDLX(name string, dlx string, maxRetries int64) error {
	if rabbit.close {
		return ErrConnectionClosed
	}

	queueArgs := amqp.Table{
		"x-dead-letter-exchange":    dlx,
		"x-dead-letter-routing-key": "dead-message",
		"x-delivery-limit":          maxRetries,
		"x-queue-type":              "quorum",
	}

	channel, err := rabbit.connection.Channel()
	if err != nil {
		return fmt.Errorf("failed to open RabbitMQ channel: %w", err)
	}

	defer channel.Close()

	_, err = channel.QueueDeclare(name, true, false, false, false, queueArgs)
	if err != nil {
		return fmt.Errorf("error declaring RabbitMQ queue: %w", err)
	}

	_, err = channel.QueueDeclare(dlx, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("error declaring RabbitMQ dlx queue: %w", err)
	}

	err = channel.ExchangeDeclare(dlx, "direct", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("error declaring RabbitMQ dlx exchange: %w", err)
	}

	err = channel.QueueBind(dlx, "dead-message", dlx, false, nil)
	if err != nil {
		return fmt.Errorf("error binding dlx queue with dlx exchange: %w", err)
	}

	return nil
}

func (rabbit *Rabbit) DeleteQueueWithDLX(name string, dlx string) error {
	errsReturn := []error{}

	deleteQueue := func() error {
		return rabbit.deleteQueueWithDLX(name, dlx)
	}

	return rabbit.retries(rabbit.maxRetries, errsReturn, deleteQueue)
}

func (rabbit *Rabbit) deleteQueueWithDLX(name string, dlx string) error {
	channel, err := rabbit.connection.Channel()
	if err != nil {
		return fmt.Errorf("failed to open RabbitMQ channel: %w", err)
	}

	defer channel.Close()

	err = channel.ExchangeDelete(dlx, false, false)
	if err != nil {
		return fmt.Errorf("error deleting dlx exchange: %w", err)
	}

	_, err = channel.QueueDelete(dlx, false, false, false)
	if err != nil {
		return fmt.Errorf("error deleting dlx queue: %w", err)
	}

	_, err = channel.QueueDelete(name, false, false, false)
	if err != nil {
		return fmt.Errorf("error deleting queue: %w", err)
	}

	return nil
}

func (rabbit *Rabbit) SendMessage(ctx context.Context, queue string, message any) error {
	errsReturn := []error{ErrEncondingMessage}

	sendMessage := func() error {
		return rabbit.sendMessage(ctx, queue, message)
	}

	return rabbit.retries(rabbit.maxRetries, errsReturn, sendMessage)
}

func (rabbit *Rabbit) sendMessage(ctx context.Context, queue string, message any) error {
	if rabbit.close {
		return ErrConnectionClosed
	}

	channel, err := rabbit.connection.Channel()
	if err != nil {
		return fmt.Errorf("failed to open RabbitMQ channel: %w", err)
	}

	defer channel.Close()

	err = channel.Confirm(false)
	if err != nil {
		return fmt.Errorf("error confirm channel: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, rabbit.timeoutSendMessage)
	defer cancel()

	messageEncoding, err := json.Marshal(message)
	if err != nil {
		return errors.Join(ErrEncondingMessage, err)
	}

	publish := amqp.Publishing{
		ContentType: "application/json",
		Body:        messageEncoding,
	}

	confirm, err := channel.PublishWithDeferredConfirmWithContext(
		ctx,
		"",
		queue,
		false,
		false,
		publish,
	)
	if err != nil {
		return errors.Join(ErrSendingMessage, err)
	}

	done, err := confirm.WaitContext(ctx)
	if err != nil {
		return errors.Join(ErrTimeoutMessage, err)
	}

	if !done {
		return ErrTimeoutMessage
	}

	return nil
}

func (rabbit *Rabbit) HandleConnection() {
	recreatDelay := time.Second

	for {
		log.Println("[INFO] - Trying to connect with RabbitMQ")

		connectionClose, err := rabbit.createConnection()
		if err != nil {
			log.Printf("[ERROR] - Error creating RabbitMQ connection: %s", err)

			time.Sleep(recreatDelay)
			recreatDelay *= 2

			continue
		}

		log.Println("[INFO] - Connection to RabbitMQ successfully established")

		recreatDelay = time.Second

		<-connectionClose

		log.Printf("[INFO] - Connection was closed, recreating connection")
	}
}

func (rabbit *Rabbit) createConnection() (chan *amqp.Error, error) {
	rabbit.close = true
	connectionClose := make(chan *amqp.Error, 1)

	connection, err := amqp.Dial(rabbit.url)
	if err != nil {
		return nil, fmt.Errorf("error creating connection: %w", err)
	}

	rabbit.connection = connection
	rabbit.connection.NotifyClose(connectionClose)

	rabbit.close = false

	return connectionClose, nil
}

func (rabbit *Rabbit) Consume(name string, bufferSize int) (<-chan amqp.Delivery, error) {
	if rabbit.close {
		return nil, ErrConnectionClosed
	}

	channel, err := rabbit.connection.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open RabbitMQ channel: %w", err)
	}

	err = channel.Qos(bufferSize, 0, false)
	if err != nil {
		return nil, fmt.Errorf("error configuring consumer queue size: %w", err)
	}

	queue, err := channel.Consume(name, "", false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("error registering consumer: %w", err)
	}

	return queue, nil
}

func New(config Config) *Rabbit {
	url := fmt.Sprintf(
		"amqp://%s:%s@%s:%s/%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Vhost,
	)

	//nolint: gomnd
	rabbit := &Rabbit{
		url:                url,
		maxRetries:         3,
		timeoutSendMessage: 5 * time.Second,
		close:              true,
	}

	return rabbit
}
