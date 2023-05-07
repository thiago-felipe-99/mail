package controllers

import (
	"errors"
	"log"
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/thiago-felipe-99/mail/publisher/core"
	"github.com/thiago-felipe-99/mail/publisher/model"
)

type User struct {
	core       *core.User
	translator *ut.UniversalTranslator
	languages  []string
}

func (controller *User) getTranslator(handler *fiber.Ctx) ut.Translator { //nolint:ireturn
	accept := handler.AcceptsLanguages(controller.languages...)
	if accept == "" {
		accept = controller.languages[0]
	}

	language, _ := controller.translator.GetTranslator(accept)

	return language
}

func (controller *User) isAdmin(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	isAdmin, err := controller.core.IsAdmin(userID)
	if err != nil {
		log.Printf("[ERROR] - error getting if user is admin: %s", err)

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error getting if user is admin"})
	}

	if !isAdmin {
		return handler.Status(fiber.StatusForbidden).JSON(sent{"current user is not admin"})
	}

	return handler.Next()
}

// Create a user session
//
//	@Summary		Create session
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Success		201		{object}	sent						"session created successfully"
//	@Failure		400		{object}	sent						"an invalid user param was sent"
//	@Failure		404		{object}	sent						"user does not exist"
//	@Failure		500		{object}	sent						"internal server error"
//	@Param			user	body		model.UserSessionPartial	true	"user params"
//	@Router			/user/session [post]
//	@Description	Create a user session and set in the response cookie.
func (controller *User) newSession(handler *fiber.Ctx) error {
	body := &model.UserSessionPartial{}

	err := handler.BodyParser(body)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	session := &model.UserSession{}

	funcCore := func() error {
		sessionTemp, err := controller.core.NewSession(*body)
		session = sessionTemp

		return err
	}

	expectErrors := []expectError{
		{core.ErrUserDoesNotExist, fiber.StatusNotFound},
		{core.ErrUserWrongPassword, fiber.StatusBadRequest},
	}

	unexpectMessageError := "error creating user session"

	okay := okay{"session created", fiber.StatusCreated}

	err = callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)

	cookie := &fiber.Cookie{
		Name:     "session",
		Value:    "",
		Expires:  time.Now(),
		HTTPOnly: true,
		Secure:   true,
	}

	deleteSession, ok := handler.Locals("deleteSession").(bool)

	if session != nil && !(ok && deleteSession) {
		cookie.Value = session.ID.String()
		cookie.Expires = session.Expires
	}

	handler.Cookie(cookie)

	return err
}

// Refresh a user session
//
//	@Summary		Refresh session
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	sent	"user session refreshed successfully"
//	@Failure		401	{object}	sent	"user session has expired"
//	@Failure		500	{object}	sent	"internal server error"
//	@Router			/user/session [put]
//	@Description	Refresh a user session and set in the response cookie.
func (controller *User) refreshSession(handler *fiber.Ctx) error {
	sessionID := handler.Cookies("session", "invalid_session")

	cookie := &fiber.Cookie{
		Name:     "session",
		Value:    "",
		Expires:  time.Now(),
		HTTPOnly: true,
		Secure:   true,
	}

	session, err := controller.core.RefreshSession(sessionID)
	if err != nil {
		handler.Cookie(cookie)

		if errors.Is(err, core.ErrUserSessionDoesNotExist) || errors.Is(err, core.ErrInvalidID) {
			return handler.Status(fiber.StatusUnauthorized).
				JSON(sent{core.ErrUserSessionDoesNotExist.Error()})
		}

		log.Printf("[ERROR] - error refreshing session: %s", err)

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	if session != nil {
		cookie.Value = session.ID.String()
		cookie.Expires = session.Expires
	}

	handler.Cookie(cookie)
	handler.Locals("userID", session.UserID)

	return handler.Next()
}

// Create a user admin
//
//	@Summary		Create admin
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Success		201		{object}	sent	"admin created successfully"
//	@Failure		400		{object}	sent	"was sent a invalid user ID"
//	@Failure		401		{object}	sent	"user session has expired"
//	@Failure		403		{object}	sent	"current user is not admin"
//	@Failure		404		{object}	sent	"user does not exist"
//	@Failure		500		{object}	sent	"internal server error"
//	@Param			userID	path		string	true	"user id to be promoted to admin"
//	@Router			/user/admin/{userID} [post]
//	@Description	Create a user admin.
func (controller *User) newAdmin(handler *fiber.Ctx) error {
	userID, err := uuid.Parse(handler.Params("userID"))
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).
			JSON(sent{"was sent a invalid user ID"})
	}

	funcCore := func() error { return controller.core.NewAdmin(userID) }

	expectErrors := []expectError{{core.ErrUserDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error promoting user"

	okay := okay{"user promoted to admin", fiber.StatusCreated}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Remove the admin role from the user
//
//	@Summary		Remove admin
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent	"admin role removed"
//	@Failure		400		{object}	sent	"was sent a invalid user ID"
//	@Failure		401		{object}	sent	"user session has expired"
//	@Failure		403		{object}	sent	"current user is not admin"
//	@Failure		404		{object}	sent	"user does not exist"
//	@Failure		500		{object}	sent	"internal server error"
//	@Param			userID	path		string	true	"user id to be removed from admin role"
//	@Router			/user/admin/{userID} [delete]
//	@Description	Remove the admin role from the user.
func (controller *User) removeAdminRole(handler *fiber.Ctx) error {
	userID, err := uuid.Parse(handler.Params("userID"))
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).
			JSON(sent{"was sent a invalid user ID"})
	}

	funcCore := func() error { return controller.core.RemoveAdmin(userID) }

	expectErrors := []expectError{
		{core.ErrUserDoesNotExist, fiber.StatusNotFound},
		{core.ErrUserIsNotAdmin, fiber.StatusNotFound},
		{core.ErrUserIsProtected, fiber.StatusForbidden},
	}

	unexpectMessageError := "error removing admin role"

	okay := okay{"admin role removed", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Create a user in application
//
//	@Summary		Create user
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Success		201		{object}	sent				"user created successfully"
//	@Failure		400		{object}	sent				"an invalid user param was sent"
//	@Failure		401		{object}	sent				"user session has expired"
//	@Failure		403		{object}	sent				"current user is not admin"
//	@Failure		409		{object}	sent				"user already exist"
//	@Failure		500		{object}	sent				"internal server error"
//	@Param			user	body		model.UserPartial	true	"user params"
//	@Router			/user [post]
//	@Description	Create a user in application.
func (controller *User) create(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	body := &model.UserPartial{}

	err := handler.BodyParser(body)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.Create(*body, userID) }

	expectErrors := []expectError{
		{core.ErrUserAlreadyExist, fiber.StatusConflict},
	}

	unexpectMessageError := "error creating user"

	okay := okay{"user created", fiber.StatusCreated}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Get current user informations
//
//	@Summary		Get user
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	sent	"user informations"
//	@Failure		401	{object}	sent	"user session has expired"
//	@Failure		404	{object}	sent	"user does not exist"
//	@Failure		500	{object}	sent	"internal server error"
//	@Router			/user [get]
//	@Description	Get current user informations.
func (controller *User) get(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	funcCore := func() (*model.User, error) {
		user, err := controller.core.GetByID(userID)
		if err != nil {
			return nil, err
		}

		user.Password = ""

		return user, nil
	}

	expectErrors := []expectError{{core.ErrUserDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error getting user"

	return callingCoreWithReturn(
		funcCore,
		expectErrors,
		unexpectMessageError,
		handler,
	)
}

// Get user by admin
//
//	@Summary		Get user
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent	"user informations"
//	@Failure		401		{object}	sent	"user session has expired"
//	@Failure		403		{object}	sent	"current user is not admin"
//	@Failure		404		{object}	sent	"user does not exist"
//	@Failure		500		{object}	sent	"internal server error"
//	@Param			userID	path		string	true	"user id"
//	@Router			/user/admin/{userID}/user [get]
//	@Description	Get user by admin.
func (controller *User) getByAdmin(handler *fiber.Ctx) error {
	userID, err := uuid.Parse(handler.Params("userID"))
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).
			JSON(sent{"was sent a invalid user ID"})
	}

	funcCore := func() (*model.User, error) {
		user, err := controller.core.GetByID(userID)
		if err != nil {
			return nil, err
		}

		user.Password = ""

		return user, nil
	}

	expectErrors := []expectError{{core.ErrUserDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error getting user"

	return callingCoreWithReturn(
		funcCore,
		expectErrors,
		unexpectMessageError,
		handler,
	)
}

// Get all users informations
//
//	@Summary		Get all users
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	sent	"user informations"
//	@Failure		401	{object}	sent	"user session has expired"
//	@Failure		403	{object}	sent	"current user is not admin"
//	@Failure		500	{object}	sent	"internal server error"
//	@Router			/user/all [get]
//	@Description	Get all users informations.
func (controller *User) getAll(handler *fiber.Ctx) error {
	funcCore := func() ([]model.User, error) {
		users, err := controller.core.GetAll()
		if err != nil {
			return nil, err
		}

		for index := range users {
			users[index].Password = ""
		}

		return users, nil
	}

	expectErrors := []expectError{}

	unexpectMessageError := "error getting all users"

	return callingCoreWithReturn(
		funcCore,
		expectErrors,
		unexpectMessageError,
		handler,
	)
}

// Update user informations
//
//	@Summary		Update user
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent				"user updated"
//	@Failure		400		{object}	sent				"an invalid user param was sent"
//	@Failure		401		{object}	sent				"user session has expired"
//	@Failure		404		{object}	sent				"user does not exist"
//	@Failure		500		{object}	sent				"internal server error"
//	@Param			user	body		model.UserPartial	true	"user params"
//	@Router			/user [put]
//	@Description	Update user informatios.
func (controller *User) update(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error updating user"})
	}

	user := &model.UserPartial{}

	err := handler.BodyParser(user)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.Update(userID, *user) }

	expectErrors := []expectError{{core.ErrUserDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error updating user"

	okay := okay{"user updated", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Delete current user
//
//	@Summary		Delete user
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	sent	"user deleted"
//	@Failure		401	{object}	sent	"user session has expired"
//	@Failure		403	{object}	sent	"current user is protected"
//	@Failure		404	{object}	sent	"user does not exist"
//	@Failure		500	{object}	sent	"internal server error"
//	@Router			/user [delete]
//	@Description	Delete current user.
func (controller *User) delete(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	funcCore := func() error { return controller.core.Delete(userID, userID) }

	expectErrors := []expectError{
		{core.ErrUserDoesNotExist, fiber.StatusNotFound},
		{core.ErrUserIsProtected, fiber.StatusForbidden},
	}

	unexpectMessageError := "error deleting user"

	okay := okay{"user deleted", fiber.StatusOK}

	handler.Locals("deleteSession", true)

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Delete user by admin
//
//	@Summary		Delete user
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent	"user deleted"
//	@Failure		401		{object}	sent	"user session has expired"
//	@Failure		403		{object}	sent	"current user is protected"
//	@Failure		404		{object}	sent	"user does not exist"
//	@Failure		500		{object}	sent	"internal server error"
//	@Param			userID	path		string	true	"user id to be deleted"
//	@Router			/user/admin/{userID}/user [delete]
//	@Description	Delete user by admin.
func (controller *User) deleteUserAdmin(handler *fiber.Ctx) error {
	adminID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	userID, err := uuid.Parse(handler.Params("userID"))
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).
			JSON(sent{"was sent a invalid user ID"})
	}

	funcCore := func() error { return controller.core.Delete(userID, adminID) }

	expectErrors := []expectError{
		{core.ErrUserDoesNotExist, fiber.StatusNotFound},
		{core.ErrUserIsProtected, fiber.StatusForbidden},
	}

	unexpectMessageError := "error deleting user"

	okay := okay{"user deleted", fiber.StatusOK}

	handler.Locals("deleteSession", true)

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Get current user roles
//
//	@Summary		Get user
//	@Tags			role, user
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	sent	"roles informations"
//	@Failure		401	{object}	sent	"user session has expired"
//	@Failure		404	{object}	sent	"user does not exist"
//	@Failure		500	{object}	sent	"internal server error"
//	@Router			/user/role [get]
//	@Description	Get current user informations.
func (controller *User) getRoles(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	funcCore := func() ([]model.UserRole, error) { return controller.core.GetRoles(userID) }

	expectErrors := []expectError{{core.ErrUserDoesNotExist, fiber.StatusNotFound}}

	unexpectMessageError := "error getting user roles"

	return callingCoreWithReturn(
		funcCore,
		expectErrors,
		unexpectMessageError,
		handler,
	)
}

//nolint:dupl
func (controller *User) hasRoles(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	roles := &[]model.UserRole{}

	err := handler.BodyParser(roles)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	hasRoles, err := controller.core.HasRoles(userID, *roles)
	if err != nil {
		log.Printf("[ERROR] - error getting if user has roles: %s", err)

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error getting if user has roles"})
	}

	if !hasRoles {
		return handler.Status(fiber.StatusForbidden).JSON(sent{"current user dont have all roles"})
	}

	return handler.Next()
}

//nolint:dupl
func (controller *User) hasRolesAdmin(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	roles := &[]model.RolePartial{}

	err := handler.BodyParser(roles)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	hasRoles, err := controller.core.HasRolesAdmin(userID, *roles)
	if err != nil {
		log.Printf("[ERROR] - error getting if user has roles: %s", err)

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error getting if user has roles"})
	}

	if !hasRoles {
		return handler.Status(fiber.StatusForbidden).JSON(sent{"current user dont have all roles"})
	}

	return handler.Next()
}

// Create a role
//
//	@Summary		Create role
//	@Tags			role, admin
//	@Accept			json
//	@Produce		json
//	@Success		201		{object}	sent				"role created successfully"
//	@Failure		400		{object}	sent				"was sent a invalid role params"
//	@Failure		401		{object}	sent				"user session has expired"
//	@Failure		403		{object}	sent				"current user is not admin"
//	@Failure		409		{object}	sent				"role already exist"
//	@Failure		500		{object}	sent				"internal server error"
//	@Param			role	body		model.RolePartial	true	"role params"
//	@Router			/user/role [post]
//	@Description	Create a role.
func (controller *User) createRole(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	role := &model.RolePartial{}

	err := handler.BodyParser(role)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.CreateRole(*role, userID) }

	expectErrors := []expectError{{core.ErrRoleAlreadyExist, fiber.StatusConflict}}

	unexpectMessageError := "error creating role"

	okay := okay{"role created", fiber.StatusCreated}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Add user roles
//
//	@Summary		Add roles
//	@Tags			role, admin
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent				"user role created successfully"
//	@Failure		400		{object}	sent				"was sent a invalid roles params"
//	@Failure		401		{object}	sent				"user session has expired"
//	@Failure		403		{object}	sent				"current user does not have all roles"
//	@Failure		404		{object}	sent				"user does not exist"
//	@Failure		500		{object}	sent				"internal server error"
//	@Param			userID	path		string				true	"user id to be promoted"
//	@Param			role	body		[]model.UserRole	true	"role params"
//	@Router			/user/role/{userID} [put]
//	@Description	Add user roles.
func (controller *User) addRoles(handler *fiber.Ctx) error {
	userID, err := uuid.Parse(handler.Params("userID"))
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).
			JSON(sent{"was sent a invalid user ID"})
	}

	roles := &[]model.UserRole{}

	err = handler.BodyParser(roles)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.AddRoles(*roles, userID) }

	expectErrors := []expectError{
		{core.ErrRoleDoesNotExist, fiber.StatusNotFound},
		{core.ErrUserDoesNotExist, fiber.StatusNotFound},
	}

	unexpectMessageError := "error adding roles"

	okay := okay{"added roles", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

func (controller *User) deleteRolesRaw(userID uuid.UUID, protected bool, handler *fiber.Ctx) error {
	roles := &[]model.RolePartial{}

	err := handler.BodyParser(roles)
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).JSON(sent{err.Error()})
	}

	funcCore := func() error { return controller.core.DeleteRoles(*roles, userID, protected) }

	expectErrors := []expectError{
		{core.ErrRoleDoesNotExist, fiber.StatusNotFound},
		{core.ErrUserDoesNotExist, fiber.StatusNotFound},
	}

	unexpectMessageError := "error deleting user roles"

	okay := okay{"deleted user roles", fiber.StatusOK}

	return callingCore(
		funcCore,
		expectErrors,
		unexpectMessageError,
		okay,
		controller.getTranslator(handler),
		handler,
	)
}

// Delete current user roles
//
//	@Summary		Delete current user roles
//	@Tags			role, user
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent				"user role deleted successfully"
//	@Failure		400		{object}	sent				"was sent a invalid role params"
//	@Failure		401		{object}	sent				"user session has expired"
//	@Failure		403		{object}	sent				"current user does not have all roles"
//	@Failure		404		{object}	sent				"user does not exist"
//	@Failure		500		{object}	sent				"internal server error"
//	@Param			role	body		[]model.RolePartial	true	"role params"
//	@Router			/user/role [delete]
//	@Description	Delete current user roles.
func (controller *User) deleteRolesByCurrentUser(handler *fiber.Ctx) error {
	userID, ok := handler.Locals("userID").(uuid.UUID)
	if !ok {
		log.Printf("[ERROR] - error getting user ID")

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{"error refreshing session"})
	}

	return controller.deleteRolesRaw(userID, true, handler)
}

// Delete user roles
//
//	@Summary		Delete user roles
//	@Tags			role, admin
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent				"user role deleted successfully"
//	@Failure		400		{object}	sent				"was sent a invalid role params"
//	@Failure		401		{object}	sent				"user session has expired"
//	@Failure		403		{object}	sent				"current user does not have all roles"
//	@Failure		404		{object}	sent				"user does not exist"
//	@Failure		500		{object}	sent				"internal server error"
//	@Param			userID	path		string				true	"user id to be deleted"
//	@Param			role	body		[]model.RolePartial	true	"role params"
//	@Router			/user/role/{userID} [delete]
//	@Description	Delete user roles.
func (controller *User) deleteRolesByRolesAdmin(handler *fiber.Ctx) error {
	userID, err := uuid.Parse(handler.Params("userID"))
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).
			JSON(sent{"was sent a invalid user ID"})
	}

	return controller.deleteRolesRaw(userID, false, handler)
}

// Delete user roles by admin
//
//	@Summary		Delete user roles by admin
//	@Tags			role, admin
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	sent				"user role deleted successfully"
//	@Failure		400		{object}	sent				"was sent a invalid role params"
//	@Failure		401		{object}	sent				"user session has expired"
//	@Failure		403		{object}	sent				"current user does not have all roles"
//	@Failure		404		{object}	sent				"user does not exist"
//	@Failure		500		{object}	sent				"internal server error"
//	@Param			userID	path		string				true	"user id to be deleted"
//	@Param			role	body		[]model.RolePartial	true	"role params"
//	@Router			/user/role/{userID}/admin [delete]
//	@Description	Delete user roles by admin.
func (controller *User) deleteRolesByAdmin(handler *fiber.Ctx) error {
	userID, err := uuid.Parse(handler.Params("userID"))
	if err != nil {
		return handler.Status(fiber.StatusBadRequest).
			JSON(sent{"was sent a invalid user ID"})
	}

	return controller.deleteRolesRaw(userID, true, handler)
}
