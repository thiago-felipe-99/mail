package controllers

import (
	"log"

	ut "github.com/go-playground/universal-translator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/thiago-felipe-99/mail/publisher/core"
	"github.com/thiago-felipe-99/mail/publisher/model"
)

type Queue struct {
	core       *core.Queue
	translator *ut.UniversalTranslator
	languages  []string
}

func (controller *Queue) getTranslator(handler *fiber.Ctx) ut.Translator { //nolint:ireturn
	accept := handler.AcceptsLanguages(controller.languages...)
	if accept == "" {
		accept = controller.languages[0]
	}

	language, _ := controller.translator.GetTranslator(accept)

	return language
}

// Create a RabbitMQ queue with DLX
//
//	@Summary		Creating queue
//	@Tags			queue
//	@Accept			json
//	@Produce		json
//	@Success		201		{object}	sent				"create queue successfully"
//	@Failure		400		{object}	sent				"an invalid queue param was sent"
//	@Failure		401		{object}	sent				"user session has expired"
//	@Failure		403		{object}	sent				"current user is not admin"
//	@Failure		409		{object}	sent				"queue already exist"
//	@Failure		500		{object}	sent				"internal server error"
//	@Param			queue	body		model.QueuePartial	true	"queue params"
//	@Router			/email/queue [post]
//	@Description	Create a RabbitMQ queue with DLX.
func (controller *Queue) create(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	body := &model.QueuePartial{
		MaxRetries: 10, //nolint:gomnd
	}

	err := handler.BodyParser(body)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.Create(*body, userID) }

	expectErrors := []expectError{{core.ErrQueueAlreadyExist, fiber.StatusConflict}}

	unexpectMessageError := "error creating queue"

	okay := okay{"queue created", fiber.StatusCreated}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Get all RabbitMQ queues
//
//	@Summary		Get queues
//	@Tags			queue
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		model.Queue	"all queues"
//	@Failure		401	{object}	sent		"user session has expired"
//	@Failure		500	{object}	sent		"internal server error"
//	@Router			/email/queue [get]
//	@Description	Get all RabbitMQ queues.
func (controller *Queue) getAll(handler *fiber.Ctx) error {
	return callingCoreWithReturn(
		controller.core.GetAll,
		[]expectError{},
		"error getting all queues",
		handler,
	)
}

// Delete a queue with DLX
//
//	@Summary		Delete queues
//	@Tags			queue
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent	"queue deleted"
//	@Failure		401		{object}	sent	"user session has expired"
//	@Failure		403		{object}	sent	"current user is not admin"
//	@Failure		404		{object}	sent	"queue does not exist"
//	@Failure		500		{object}	sent	"internal server error"
//	@Param			name	path		string	true	"queue name"
//	@Router			/email/queue/{name} [delete]
//	@Description	Delete a queue with DLX.
func (controller *Queue) delete(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	funcCore := func() error { return controller.core.Delete(handler.Params("name"), userID) }

	expectErrors := []expectError{{core.ErrQueueDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error deleting queue"

	okay := okay{"queue deleted", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Sends an email to the RabbitMQ queue
//
//	@Summary		Sends email
//	@Tags			queue
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent		"email sent successfully"
//	@Failure		400		{object}	sent		"an invalid email param was sent"
//	@Failure		401		{object}	sent		"user session has expired"
//	@Failure		403		{object}	sent		"current user is not admin"
//	@Failure		404		{object}	sent		"queue does not exist"
//	@Failure		500		{object}	sent		"internal server error"
//	@Param			name	path		string		true	"queue name"
//	@Param			queue	body		model.Email	true	"email"
//	@Router			/email/queue/{name}/send [post]
//	@Description	Sends an email to the RabbitMQ queue.
func (controller *Queue) sendEmail(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	body := &model.EmailPartial{}

	err := handler.BodyParser(body)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.SendEmail(handler.Params("name"), *body, userID) }

	expectErrors := []expectError{
		{core.ErrQueueDoesNotExist, fiber.StatusNotFound},
		{core.ErrMissingFieldTemplates, fiber.StatusBadRequest},
		{core.ErrTemplateDoesNotExist, fiber.StatusBadRequest},
	}

	unexpectMessageError := "error sending email"

	okay := okay{"email sent", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}
