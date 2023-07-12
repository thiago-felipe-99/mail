package controllers

import (
	"log"

	ut "github.com/go-playground/universal-translator"
	"github.com/gofiber/fiber/v2"
	"github.com/thiago-felipe-99/mail/publisher/core"
	"github.com/thiago-felipe-99/mail/publisher/model"
)

type EmailList struct {
	core       *core.EmailList
	translator *ut.UniversalTranslator
	languages  []string
}

func (controller *EmailList) getTranslator(handler *fiber.Ctx) ut.Translator { //nolint:ireturn
	accept := handler.AcceptsLanguages(controller.languages...)
	if accept == "" {
		accept = controller.languages[0]
	}

	language, _ := controller.translator.GetTranslator(accept)

	return language
}

// Create a email list to user.
//
//	@Summary		Creating email list
//	@Tags			emailList
//	@Accept			json
//	@Produce		json
//	@Success		201			{object}	sent					"create email list successfully"
//	@Failure		400			{object}	sent					"an invalid email list param was sent"
//	@Failure		401			{object}	sent					"user session has expired"
//	@Failure		409			{object}	sent					"email list already exist"
//	@Failure		500			{object}	sent					"internal server error"
//	@Param			emailList	body		model.EmailListPartial	true	"email list params"
//	@Router			/email/list [post]
//	@Description	Create a email list to user.
func (controller *EmailList) create(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(model.ID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	body := &model.EmailListPartial{}

	err := handler.BodyParser(body)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.Create(userID, *body) }

	expectErrors := []expectError{{core.ErrEmailListAlreadyExist, fiber.StatusConflict}}

	unexpectMessageError := "error creating email list"

	okay := okay{"email list created", fiber.StatusCreated}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Get a user email list
//
//	@Summary		Get a user email list
//	@Tags			emailList
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	model.EmailList	"email list"
//	@Failure		401		{object}	sent			"user session has expired"
//	@Failure		404		{object}	sent			"email list not found"
//	@Failure		500		{object}	sent			"internal server error"
//	@Param			name	path		string			true	"email list name"
//	@Router			/email/list/{name} [get]
//	@Description	Get a user email list.
func (controller *EmailList) get(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(model.ID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	funcCore := func() (*model.EmailList, error) { return controller.core.Get(handler.Params("name"), userID) }

	expectErrors := []expectError{{core.ErrEmailListDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error getting user email list"

	return callingCoreWithReturn(
		funcCore,
		expectErrors,
		unexpectMessageError,
		controller.getTranslator(handler),
		handler,
	)
}

// Get all user email list
//
//	@Summary		Get all user email list
//	@Tags			emailList
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		model.EmailList	"email list"
//	@Failure		401	{object}	sent			"user session has expired"
//	@Failure		500	{object}	sent			"internal server error"
//	@Router			/email/list [get]
//	@Description	Get all user email list.
func (controller *EmailList) getAll(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(model.ID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	funcCore := func() ([]model.EmailList, error) { return controller.core.GetAll(userID) }

	expectErrors := []expectError{}

	unexpectMessageError := "error getting user email list"

	return callingCoreWithReturn(
		funcCore,
		expectErrors,
		unexpectMessageError,
		controller.getTranslator(handler),
		handler,
	)
}

// Update information for an email list.
//
//	@Summary		Update info email list
//	@Tags			emailList
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent				"update email list successfully"
//	@Failure		400		{object}	sent				"an invalid email list param was sent"
//	@Failure		401		{object}	sent				"user session has expired"
//	@Failure		404		{object}	sent				"email list does not exist"
//	@Failure		409		{object}	sent				"email list already exist"
//	@Failure		500		{object}	sent				"internal server error"
//	@Param			name	path		string				true	"email list name"
//	@Param			info	body		model.EmailListInfo	true	"email list info"
//	@Router			/email/list/{name} [put]
//	@Description	Update information for an email list.
func (controller *EmailList) updateInfo(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(model.ID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	body := &model.EmailListInfo{}

	err := handler.BodyParser(body)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.UpdateInfo(handler.Params("name"), userID, *body) }

	expectErrors := []expectError{
		{core.ErrEmailListDoesNotExist, fiber.StatusNotFound},
		{core.ErrEmailListAlreadyExist, fiber.StatusConflict},
	}

	unexpectMessageError := "error updating email list"

	okay := okay{"email list updated", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Delete an email list.
//
//	@Summary		Delete email list
//	@Tags			emailList
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent	"delete email list successfully"
//	@Failure		401		{object}	sent	"user session has expired"
//	@Failure		404		{object}	sent	"email list does not exist"
//	@Failure		500		{object}	sent	"internal server error"
//	@Param			name	path		string	true	"email list name"
//	@Router			/email/list/{name} [delete]
//	@Description	Delete an email list.
func (controller *EmailList) delete(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(model.ID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	funcCore := func() error { return controller.core.Delete(handler.Params("name"), userID, userID) }

	expectErrors := []expectError{{core.ErrEmailListDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error deleting email list"

	okay := okay{"email list deleted", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Add emails to email list.
//
//	@Summary		Add emails to email list
//	@Tags			emailList
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent					"update email list successfully"
//	@Failure		400		{object}	sent					"an invalid email param was sent"
//	@Failure		401		{object}	sent					"user session has expired"
//	@Failure		404		{object}	sent					"email list does not exist"
//	@Failure		500		{object}	sent					"internal server error"
//	@Param			name	path		string					true	"email list name"
//	@Param			emails	body		model.EmailListEmails	true	"emails"
//	@Router			/email/list/{name}/add [post]
//	@Description	Add emails to email list.
func (controller *EmailList) addEmail(handler *fiber.Ctx) error { //nolint: dupl
	userID, ok := handler.Locals("userID").(model.ID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	body := &model.EmailListEmails{}

	err := handler.BodyParser(body)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.AddEmails(handler.Params("name"), userID, *body) }

	expectErrors := []expectError{{core.ErrEmailListDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error adding emails"

	okay := okay{"emails added", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Remove emails to email list.
//
//	@Summary		Remove emails to email list
//	@Tags			emailList
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent					"update email list successfully"
//	@Failure		400		{object}	sent					"an invalid email param was sent"
//	@Failure		401		{object}	sent					"user session has expired"
//	@Failure		404		{object}	sent					"email list does not exist"
//	@Failure		500		{object}	sent					"internal server error"
//	@Param			name	path		string					true	"email list name"
//	@Param			emails	body		model.EmailListEmails	true	"emails"
//	@Router			/email/list/{name}/remove [delete]
//	@Description	Remove emails to email list.
func (controller *EmailList) removeEmails(handler *fiber.Ctx) error { //nolint: dupl
	userID, ok := handler.Locals("userID").(model.ID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	body := &model.EmailListEmails{}

	err := handler.BodyParser(body)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.RemoveEmails(handler.Params("name"), userID, *body) }

	expectErrors := []expectError{{core.ErrEmailListDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error removing emails"

	okay := okay{"emails removed", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Remove email to email list.
//
//	@Summary		Remove email to email list
//	@Tags			emailList
//	@Accept			json
//	@Produce		json
//	@Success		200			{object}	sent	"update email list successfully"
//	@Failure		404			{object}	sent	"email list does not exist"
//	@Failure		500			{object}	sent	"internal server error"
//	@Param			user_id		path		string	true	"user id from emailist"
//	@Param			name		path		string	true	"email list name"
//	@Param			email_id	path		string	true	"email id from emailist"
//	@Router			/email/list/{user_id}/{name}/{email_id} [delete]
//	@Description	Remove emails to email list.
func (controller *EmailList) removeEmail(handler *fiber.Ctx) error {
	userID, err := model.ParseID(handler.Params("user_id"))
	if err != nil {
		return handler.Status(fiber.StatusNotFound).
			JSON(sent{core.ErrEmailListDoesNotExist.Error()})
	}

	emailID, err := model.ParseID(handler.Params("email_id"))
	if err != nil {
		return handler.Status(fiber.StatusNotFound).
			JSON(sent{core.ErrEmailListDoesNotExist.Error()})
	}

	funcCore := func() error { return controller.core.RemoveEmail(handler.Params("name"), userID, emailID) }

	expectErrors := []expectError{{core.ErrEmailListDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error removing email"

	okay := okay{"email removed", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}
