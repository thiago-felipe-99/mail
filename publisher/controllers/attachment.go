package controllers

import (
	"log"

	ut "github.com/go-playground/universal-translator"
	"github.com/gofiber/fiber/v2"
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

// Create a upload attachment url
//
//	@Summary		Creating attachment
//	@Tags			attachment
//	@Accept			json
//	@Produce		json
//	@Success		200			{object}	model.AttachmentURL		"create attachment successfully"
//	@Failure		400			{object}	sent					"an invalid attachment param was sent"
//	@Failure		401			{object}	sent					"user session has expired"
//	@Failure		500			{object}	sent					"internal server error"
//	@Param			attachment	body		model.AttachmentPartial	true	"attachment params"
//	@Router			/email/attachment [post]
//	@Description	Create a upload attachment url.
func (controller *Attachment) create(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(model.ID)
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

	funcCore := func() (*model.AttachmentURL, error) { return controller.core.Create(*body, userID) }

	expectErrors := []expectError{{core.ErrMaxSizeAttachment, fiber.StatusBadRequest}}

	unexpectMessageError := "error creating attachment url"

	return callingCoreWithReturn(
		funcCore,
		expectErrors,
		unexpectMessageError,
		controller.getTranslator(handler),
		handler,
	)
}

// Get a download attachment url
//
//	@Summary		Get attachment url
//	@Tags			attachment
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	model.AttachmentURL	"attachment url"
//	@Failure		400	{object}	sent				"was sent a invalid attachment ID"
//	@Failure		401	{object}	sent				"user session has expired"
//	@Failure		404	{object}	sent				"attachment does not exist"
//	@Failure		500	{object}	sent				"internal server error"
//	@Param			id	path		string				true	"attachment id"
//	@Router			/email/attachment/{id} [get]
//	@Description	Get a download attachment url.
func (controller *Attachment) get(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(model.ID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	attachmentID, err := model.ParseID(handler.Params("id"))
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).
			JSON(sent{"was sent a invalid attachment ID"})
	}

	funcCore := func() (*model.AttachmentURL, error) { return controller.core.Get(attachmentID, userID) }

	expectErrors := []expectError{{core.ErrAttachmentDoesNotExist, fiber.StatusNotFound}}

	return callingCoreWithReturn(
		funcCore,
		expectErrors,
		"error getting attachment",
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
//	@Router			/email/attachment [get]
//	@Description	Get all user attachments.
func (controller *Attachment) getAttachments(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(model.ID)
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

// Confirm upload
//
//	@Summary		Confirm upload
//	@Tags			attachment
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	sent	"upload confimed"
//	@Failure		400	{object}	sent	"was sent a invalid attachment ID"
//	@Failure		401	{object}	sent	"user session has expired"
//	@Failure		404	{object}	sent	"attachment does not exist"
//	@Failure		500	{object}	sent	"internal server error"
//	@Param			id	path		string	true	"attachment id"
//	@Router			/email/attachment/{id}/confirm [post]
//	@Description	Confirm upload.
func (controller *Attachment) confirm(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(model.ID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	attachmentID, err := model.ParseID(handler.Params("id"))
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).
			JSON(sent{"was sent a invalid attachment ID"})
	}

	funcCore := func() error { return controller.core.ConfirmUpload(attachmentID, userID) }

	expectErrors := []expectError{
		{core.ErrAttachmentDoesNotExist, fiber.StatusNotFound},
		{core.ErrAttachmentDoesNotExistOnMinio, fiber.StatusNotFound},
	}

	okay := okay{"upload confirmed", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		"error refreshing upload URL",
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Refresh a upload attachment url
//
//	@Summary		Refresh attachment url
//	@Tags			attachment
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	model.AttachmentURL	"attachment url"
//	@Failure		400	{object}	sent				"was sent a invalid attachment ID"
//	@Failure		401	{object}	sent				"user session has expired"
//	@Failure		404	{object}	sent				"attachment does not exist"
//	@Failure		500	{object}	sent				"internal server error"
//	@Param			id	path		string				true	"attachment id"
//	@Router			/email/attachment/{id} [post]
//	@Description	Refresh a upload attachment url.
func (controller *Attachment) refresh(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(model.ID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	attachmentID, err := model.ParseID(handler.Params("id"))
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).
			JSON(sent{"was sent a invalid attachment ID"})
	}

	funcCore := func() (*model.AttachmentURL, error) { return controller.core.RefreshUploadURL(attachmentID, userID) }

	expectErrors := []expectError{
		{core.ErrAttachmentDoesNotExist, fiber.StatusNotFound},
		{core.ErrUploadAlreadyConfirmed, fiber.StatusConflict},
	}

	return callingCoreWithReturn(
		funcCore,
		expectErrors,
		"error refreshing upload URL",
		controller.getTranslator(handler),
		handler,
	)
}
