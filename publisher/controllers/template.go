package controllers

import (
	"log"

	ut "github.com/go-playground/universal-translator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/thiago-felipe-99/mail/publisher/core"
	"github.com/thiago-felipe-99/mail/publisher/model"
)

type Template struct {
	core       *core.Template
	translator *ut.UniversalTranslator
	languages  []string
}

func (controller *Template) getTranslator(handler *fiber.Ctx) ut.Translator { //nolint:ireturn
	accept := handler.AcceptsLanguages(controller.languages...)
	if accept == "" {
		accept = controller.languages[0]
	}

	language, _ := controller.translator.GetTranslator(accept)

	return language
}

// Create a email template
//
//	@Summary		Creating template
//	@Tags			template
//	@Accept			json
//	@Produce		json
//	@Success		201			{object}	sent					"create template successfully"
//	@Failure		400			{object}	sent					"an invalid template param was sent"
//	@Failure		401			{object}	sent					"user session has expired"
//	@Failure		409			{object}	sent					"template name already exist"
//	@Failure		500			{object}	sent					"internal server error"
//	@Param			template	body		model.TemplatePartial	true	"template params"
//	@Router			/email/template [post]
//	@Description	Create a email template.
func (controller *Template) create(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	body := &model.TemplatePartial{}

	err := handler.BodyParser(body)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.Create(*body, userID) }

	expectErrors := []expectError{
		{core.ErrTemplateNameAlreadyExist, fiber.StatusConflict},
		{core.ErrMaxSizeTemplate, fiber.StatusBadRequest},
	}

	unexpectMessageError := "error creating template"

	okay := okay{"template created", fiber.StatusCreated}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Get all email templates
//
//	@Summary		Get templates
//	@Tags			template
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		model.Template	"all templates"
//	@Failure		401	{object}	sent			"user session has expired"
//	@Failure		500	{object}	sent			"internal server error"
//	@Router			/email/template [get]
//	@Description	Get all email templates.
func (controller *Template) getAll(handler *fiber.Ctx) error {
	return callingCoreWithReturn(
		controller.core.GetAll,
		[]expectError{},
		"error getting all templates",
		handler,
	)
}

// Get a email template
//
//	@Summary		Get template
//	@Tags			template
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	model.Template	"all templates"
//	@Failure		401		{object}	sent			"user session has expired"
//	@Success		404		{array}		sent			"template does not exist"
//	@Failure		500		{object}	sent			"internal server error"
//	@Param			name	path		string			true	"template name"
//	@Router			/email/template/{name} [get]
//	@Description	Get a email template.
func (controller *Template) get(handler *fiber.Ctx) error {
	coreFunc := func() (*model.Template, error) { return controller.core.Get(handler.Params("name")) }

	expectErros := []expectError{{core.ErrTemplateDoesNotExist, fiber.StatusNotFound}}

	return callingCoreWithReturn(coreFunc, expectErros, "error getting template", handler)
}

// Update a email template
//
//	@Summary		Update template
//	@Tags			template
//	@Accept			json
//	@Produce		json
//	@Success		200			{object}	sent					"template updated"
//	@Failure		400			{object}	sent					"an invalid template param was sent"
//	@Failure		401			{object}	sent					"user session has expired"
//	@Failure		404			{object}	sent					"template does not exist"
//	@Failure		500			{object}	sent					"internal server error"
//	@Param			name		path		string					true	"template name"
//	@Param			template	body		model.TemplatePartial	true	"template params"
//	@Router			/email/template/{name} [put]
//	@Description	Update a email template.
func (controller *Template) update(handler *fiber.Ctx) error {
	body := &model.TemplatePartial{}

	err := handler.BodyParser(body)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.Update(handler.Params("name"), *body) }

	expectErrors := []expectError{
		{core.ErrTemplateDoesNotExist, fiber.StatusNotFound},
		{core.ErrMaxSizeTemplate, fiber.StatusBadRequest},
	}

	unexpectMessageError := "error updating template"

	okay := okay{"template updated", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Delete a email template
//
//	@Summary		Delete template
//	@Tags			template
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent	"template deleted"
//	@Failure		401		{object}	sent	"user session has expired"
//	@Failure		404		{object}	sent	"template does not exist"
//	@Failure		500		{object}	sent	"internal server error"
//	@Param			name	path		string	true	"template name"
//	@Router			/email/template/{name} [delete]
//	@Description	Delete a email template.
func (controller *Template) delete(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	funcCore := func() error { return controller.core.Delete(handler.Params("name"), userID) }

	expectErrors := []expectError{{core.ErrTemplateDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error deleting template"

	okay := okay{"template deleted", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}
