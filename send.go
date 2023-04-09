package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/wneessen/go-mail"
)

type receiver struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type email struct {
	Receiver        receiver         `json:"receiver"`
	Subject         string           `json:"subject"`
	Message         string           `json:"message"`
	Type            mail.ContentType `json:"type"`
	Attachments     []string         `json:"attachments"`
	attachmentsSize int
	messageRabbit   amqp.Delivery
}

type send struct {
	*cache
	*sender
	*metrics
	client *mail.Client
	infos  chan string
	errors chan string
}

func newSend(cache *cache, sender *sender, smtp *smtp, metrics *metrics) (*send, error) {
	clientOption := []mail.Option{
		mail.WithPort(smtp.Port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(smtp.User),
		mail.WithPassword(smtp.Password),
		mail.WithTLSPolicy(mail.TLSMandatory),
	}

	client, err := mail.NewClient(smtp.Host, clientOption...)
	if err != nil {
		return nil, fmt.Errorf("error creating an email client: %w", err)
	}

	return &send{
		cache:   cache,
		sender:  sender,
		metrics: metrics,
		client:  client,
		infos:   make(chan string),
		errors:  make(chan string),
	}, nil
}

func (send *send) messageToQueue(message amqp.Delivery) {
	send.metrics.emailsResent.Inc()

	err := message.Nack(false, true)
	if err != nil {
		send.errors <- fmt.Sprintf("Error resending message to the queue: %s", err)
	}
}

func (send *send) queueToEmails(queue []amqp.Delivery) ([]email, int) {
	emails := []email{}
	bytesReceived := 0

	for _, message := range queue {
		bytesReceived += len(message.Body)

		email := email{}

		err := json.Unmarshal(message.Body, &email)
		if err != nil {
			send.errors <- fmt.Sprintf("Error converting a message to an email: %s", err)
			send.messageToQueue(message)
		} else {
			email.messageRabbit = message
			email.attachmentsSize = 0
			emails = append(emails, email)
		}
	}

	return emails, bytesReceived
}

func (send *send) emailsToMessages(emails []email) ([]*mail.Msg, []email) {
	messages := []*mail.Msg{}
	emailsReady := []email{}

emailToMessage:
	for _, email := range emails {
		message := mail.NewMsg()

		err := message.EnvelopeFromFormat(send.sender.Name, send.sender.Email)
		if err != nil {
			send.errors <- fmt.Sprintf("Error adding email sender: %s", err)
			send.messageToQueue(email.messageRabbit)

			continue
		}

		err = message.AddToFormat(email.Receiver.Name, email.Receiver.Email)
		if err != nil {
			send.errors <- fmt.Sprintf("Error adding email receiver: %s", err)
			send.messageToQueue(email.messageRabbit)

			continue
		}

		for _, attachment := range email.Attachments {
			file, err := send.cache.getFile(attachment)
			if err != nil {
				send.errors <- fmt.Sprintf("Error getting file from cache: %s", err)
				send.messageToQueue(email.messageRabbit)

				continue emailToMessage
			}

			email.attachmentsSize += len(file)
			message.AttachReadSeeker(attachment, bytes.NewReader(file))
		}

		message.Subject(email.Subject)
		message.SetBodyString(email.Type, email.Message)

		messages = append(messages, message)
		emailsReady = append(emailsReady, email)
	}

	return messages, emailsReady
}

func (send *send) emails(queue []amqp.Delivery) {
	timeInit := time.Now()

	emails, bytesReceived := send.queueToEmails(queue)

	send.metrics.emailsReceived.Add(float64(len(queue)))
	send.metrics.emailsReceivedBytes.Add(float64(bytesReceived))

	messages, emailsReady := send.emailsToMessages(emails)
	if len(messages) == 0 {
		return
	}

	err := send.client.DialAndSend(messages...)
	if err != nil {
		send.errors <- fmt.Sprintf("Error processing a batch of emails: %s", err)

		for _, email := range emails {
			send.messageToQueue(email.messageRabbit)
		}

		return
	}

	sentEmails := 0
	sentBytes := 0
	sentAttachment := 0
	sentAttachmentsBytes := 0
	sentWithAttachemnt := 0

	for _, email := range emailsReady {
		err := email.messageRabbit.Ack(false)
		if err != nil {
			send.errors <- fmt.Sprintf("Error sending a termination message to RabbitMQ: %s", err)
		} else {
			sentEmails++
			sentBytes += len(email.Message)

			attachmentsSize := len(email.Attachments)
			if attachmentsSize > 0 {
				sentAttachment += attachmentsSize
				sentAttachmentsBytes += email.attachmentsSize
				sentWithAttachemnt++
			}
		}
	}

	send.metrics.emailsSentTimeSeconds.Observe(time.Since(timeInit).Seconds())
	send.metrics.emailsSent.Add(float64(sentEmails))
	send.metrics.emailsSentBytes.Add(float64(sentBytes))
	send.metrics.emailsSentAttachment.Add(float64(sentAttachment))
	send.metrics.emailsSentAttachmentBytes.Add(float64(sentAttachmentsBytes))
	send.metrics.emailsSentWithAttachment.Add(float64(sentWithAttachemnt))

	send.infos <- fmt.Sprintf("Has been sent %d emails", sentEmails)
}

func (send *send) copyQueueAndSendEmails(queue []amqp.Delivery) []amqp.Delivery {
	buffer := make([]amqp.Delivery, len(queue))
	copy(buffer, queue)

	send.infos <- fmt.Sprintf("Sending %d emails", len(buffer))

	go send.emails(buffer)

	return queue[:0]
}
