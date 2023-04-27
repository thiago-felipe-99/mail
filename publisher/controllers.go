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

type receiver struct {
	Name  string `json:"name"  validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type template struct {
	Name string            `json:"name" validate:"required"`
	Data map[string]string `json:"data"`
}

type email struct {
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

func (queue *queue) bodyParser(body any, handler *fiber.Ctx) (int, error) {
	err := handler.BodyParser(body)
	if err != nil {
		return fiber.StatusBadRequest, errBodyValidate
	}

	err = queue.validate.Struct(body)
	if err != nil {
		validationErrs := validator.ValidationErrors{}

		okay := errors.As(err, &validationErrs)
		if !okay {
			return fiber.StatusBadRequest, errBodyValidate
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

		return fiber.StatusBadRequest, errors.New(messageSend) //nolint: goerr113
	}

	return 0, nil
}

func (queue *queue) create() func(*fiber.Ctx) error {
	return func(handler *fiber.Ctx) error {
		body := &struct {
			Name       string `json:"name" validate:"required"`
			MaxRetries int64  `json:"maxRetries"`
		}{
			MaxRetries: 10, //nolint:gomnd
		}

		status, err := queue.bodyParser(body, handler)
		if err != nil {
			return handler.Status(status).SendString(err.Error())
		}

		if queue.queues.Exist(body.Name) {
			return handler.Status(fiber.StatusConflict).SendString(errQueueAlreadyExist.Error())
		}

		err = queue.rabbit.CreateQueue(body.Name, body.MaxRetries)
		if err != nil {
			log.Printf("[ERROR] - Error creating queue: %s", err)

			return handler.Status(fiber.StatusInternalServerError).
				SendString("error creating queue")
		}

		queue.queues.Add(body.Name)

		return nil
	}
}

func (queue *queue) send() func(*fiber.Ctx) error {
	return func(handler *fiber.Ctx) error {
		name := handler.Params("name")

		if !queue.queues.Exist(name) {
			return handler.Status(fiber.StatusNotFound).SendString(errQueueDontExist.Error())
		}

		body := &email{}

		status, err := queue.bodyParser(body, handler)
		if err != nil {
			return handler.Status(status).SendString(err.Error())
		}

		err = queue.rabbit.SendMessage(context.Background(), name, body)
		if err != nil {
			log.Printf("[ERROR] - Error creating queue: %s", err)

			return handler.Status(fiber.StatusInternalServerError).
				SendString("error creating queue")
		}

		return nil
	}
}