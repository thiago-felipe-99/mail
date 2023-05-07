package controllers

import (
	"log"

	ut "github.com/go-playground/universal-translator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/thiago-felipe-99/mail/publisher/core"
	"github.com/thiago-felipe-99/mail/publisher/model"
)

type Attachment struct {
	core       *core.Attachment
	translator *ut.UniversalTranslator
	languages  []string
}

func (controller *Attachment) getTranslator(handler *fiber.Ctx) ut.Translator { //nolint:ireturn
	accept := handler.AcceptsLanguages(controller.languages...)
	if accept == "" {
		accept = controller.languages[0]
	}

	language, _ := controller.translator.GetTranslator(accept)

	return language
}

// Create a attachment link
//
//	@Summary		Creating attachment
//	@Tags			attachment
//	@Accept			json
//	@Produce		json
//	@Success		200			{object}	model.AttachmentLink	"create attachment successfully"
//	@Failure		400			{object}	sent					"an invalid attachment param was sent"
//	@Failure		401			{object}	sent					"user session has expired"
//	@Failure		500			{object}	sent					"internal server error"
//	@Param			attachment	body		model.AttachmentPartial	true	"attachment params"
//	@Router			/email/attachment [post]
//	@Description	Create a attachment link.
func (controller *Attachment) create(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	body := &model.AttachmentPartial{}

	err := handler.BodyParser(body)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() (*model.AttachmentLink, error) { return controller.core.Create(*body, userID) }

	expectErrors := []expectError{}

	unexpectMessageError := "error creating attachment link"

	return callingCoreWithReturn(
		funcCore,
		expectErrors,
		unexpectMessageError,
		controller.getTranslator(handler),
		handler,
	)
}

// Get all user attachments
//
//	@Summary		Get user attachments
//	@Tags			attachment
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		model.Attachment	"all attachments"
//	@Failure		401	{object}	sent				"user session has expired"
//	@Failure		500	{object}	sent				"internal server error"
//	@Router			/email/attachment/user [get]
//	@Description	Get all user attachments.
func (controller *Attachment) getAttachments(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	funcCore := func() ([]model.Attachment, error) { return controller.core.GetAttachments(userID) }

	return callingCoreWithReturn(
		funcCore,
		[]expectError{},
		"error getting all attachments",
		controller.getTranslator(handler),
		handler,
	)
}