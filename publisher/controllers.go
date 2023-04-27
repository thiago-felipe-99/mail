//nolint:wrapcheck
package main

import (
	"context"
	"errors"
	"log"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/thiago-felipe-99/mail/rabbit"
)

var (
	errQueueAlreadyExist = errors.New("queue already exist")
	errQueueDontExist    = errors.New("queue dont exist")
	errBodyValidate      = errors.New("unable to parse body")
)

type sent struct {
	Message string `json:"message"`
}

type receiver struct {
	Name  string `json:"name"  validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type template struct {
	Name string            `json:"name" validate:"required"`
	Data map[string]string `json:"data"`
}

type emailBody struct {
	Receivers      []receiver `json:"receivers"      validate:"required_without=BlindReceivers"`
	BlindReceivers []receiver `json:"blindReceivers" validate:"required_without=Receivers"`
	Subject        string     `json:"subject"        validate:"required"`
	Message        string     `json:"message"        validate:"required_without=Template,excluded_with=Template"`
	Template       *template  `json:"template"       validate:"required_without=Message,excluded_with=Message"`
	Attachments    []string   `json:"attachments"`
}

type queue struct {
	rabbit     *rabbit.Rabbit
	queues     *rabbit.Queues
	validate   *validator.Validate
	translator *ut.UniversalTranslator
	languages  []string
}

func (queue *queue) bodyParser(body any, handler *fiber.Ctx) error {
	err := handler.BodyParser(body)
	if err != nil {
		return errBodyValidate
	}

	err = queue.validate.Struct(body)
	if err != nil {
		validationErrs := validator.ValidationErrors{}

		okay := errors.As(err, &validationErrs)
		if !okay {
			return errBodyValidate
		}

		accept := handler.AcceptsLanguages(queue.languages...)
		if accept == "" {
			accept = queue.languages[0]
		}

		language, _ := queue.translator.GetTranslator(accept)

		messages := validationErrs.Translate(language)

		messageSend := ""
		for _, message := range messages {
			messageSend += "\n" + message
		}

		return errors.New(messageSend) //nolint: goerr113
	}

	return nil
}

type queueBody struct {
	Name       string `json:"name"       validate:"required"`
	MaxRetries int64  `json:"maxRetries"`
}

// Creating a RabbitMQ queue
//
// @Summary		Creating queue
// @Tags			queue
// @Accept			json
// @Produce		json
// @Success		200		{object}	sent "create queue successfully"
// @Failure		400		{object}	sent "an invalid queue param was sent"
// @Failure		409		{object}	sent "queue already exist"
// @Failure		500		{object}	sent "internal server error"
// @Param			queue	body		queueBody	true	"queue params"
// @Router			/email/queue [post]
// @Description	Creating a RabbitMQ queue.
func (queue *queue) create() func(*fiber.Ctx) error {
	return func(handler *fiber.Ctx) error {
		body := &queueBody{
			MaxRetries: 10, //nolint:gomnd
		}

		err := queue.bodyParser(body, handler)
		if err != nil {
			return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
		}

		if queue.queues.Exist(body.Name) {
			return handler.Status(fiber.StatusConflict).JSON(sent{errQueueAlreadyExist.Error()})
		}

		err = queue.rabbit.CreateQueue(body.Name, body.MaxRetries)
		if err != nil {
			log.Printf("[ERROR] - Error creating queue: %s", err)

			return handler.Status(fiber.StatusInternalServerError).
				JSON(sent{"error creating queue"})
		}

		queue.queues.Add(body.Name)

		return handler.Status(fiber.StatusCreated).JSON(sent{"Queue created"})
	}
}

// Sends an email to the RabbitMQ queue
//
// @Summary		Sends email
// @Tags			queue
// @Accept			json
// @Produce		json
// @Success		200		{object}	sent "email sent successfully"
// @Failure		400		{object}	sent "an invalid email param was sent"
// @Failure		404		{object}	sent "queue does not exist"
// @Failure		500		{object}	sent "internal server error"
// @Param			name	path	string		true	"queue name"
// @Param			queue	body	emailBody	true	"email"
// @Router			/email/queue/{name}/send [post]
// @Description	Sends an email to the RabbitMQ queue.
func (queue *queue) send() func(*fiber.Ctx) error {
	return func(handler *fiber.Ctx) error {
		name := handler.Params("name")

		if !queue.queues.Exist(name) {
			return handler.Status(fiber.StatusNotFound).JSON(sent{errQueueDontExist.Error()})
		}

		body := &emailBody{}

		err := queue.bodyParser(body, handler)
		if err != nil {
			return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
		}

		err = queue.rabbit.SendMessage(context.Background(), name, body)
		if err != nil {
			log.Printf("[ERROR] - Error creating queue: %s", err)

			return handler.Status(fiber.StatusInternalServerError).
				JSON(sent{"error creating queue"})
		}

		return handler.JSON(sent{"Email sent"})
	}
}
