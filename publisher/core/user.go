package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/go-playground/validator/v10"
	"github.com/thiago-felipe-99/mail/publisher/data"
	"github.com/thiago-felipe-99/mail/publisher/model"
)

type User struct {
	database        *data.User
	validator       *validator.Validate
	argon2id        argon2id.Params
	durationSession time.Duration
}

func (core *User) existByID(userID model.ID) (bool, error) {
	exist, err := core.database.ExistByID(userID)
	if err != nil {
		return false, fmt.Errorf("error checking if user exist in database: %w", err)
	}

	return exist, nil
}

func (core *User) ExistByNameOrEmail(name, email string) (bool, error) {
	exist, err := core.database.ExistByNameOrEmail(name, email)
	if err != nil {
		return false, fmt.Errorf("error checking if user exist in database: %w", err)
	}

	return exist, nil
}

func (core *User) Create(partial model.UserPartial, adminID model.ID) error {
	err := validate(core.validator, partial)
	if err != nil {
		return err
	}

	exist, err := core.ExistByNameOrEmail(partial.Name, partial.Email)
	if err != nil {
		return fmt.Errorf("error checking if user exist in database: %w", err)
	}

	if exist {
		return ErrUserAlreadyExist
	}

	hash, err := argon2id.CreateHash(partial.Password, &core.argon2id)
	if err != nil {
		return fmt.Errorf("error creating password hash: %w", err)
	}

	user := model.User{
		ID:        model.NewID(),
		Name:      partial.Name,
		Email:     partial.Email,
		Password:  hash,
		CreatedAt: time.Now(),
		CreatedBy: adminID,
		IsAdmin:   false,
		DeletedAt: time.Time{},
		DeletedBy: model.ID{},
	}

	err = core.database.Create(user)
	if err != nil {
		return fmt.Errorf("error creating user in database: %w", err)
	}

	return nil
}

func (core *User) GetByID(userID model.ID) (*model.User, error) {
	exist, err := core.existByID(userID)
	if err != nil {
		return nil, fmt.Errorf("error checking if user exist in database: %w", err)
	}

	if !exist {
		return nil, ErrUserDoesNotExist
	}

	user, err := core.database.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("error getting user from database: %w", err)
	}

	return user, nil
}

func (core *User) GetAll() ([]model.User, error) {
	users, err := core.database.GetAll()
	if err != nil {
		return nil, fmt.Errorf("error getting alls users from database: %w", err)
	}

	return users, nil
}

func (core *User) GetByNameOrEmail(name, email string) (*model.User, error) {
	exist, err := core.ExistByNameOrEmail(name, email)
	if err != nil {
		return nil, fmt.Errorf("error checking if user exist in database: %w", err)
	}

	if !exist {
		return nil, ErrUserDoesNotExist
	}

	user, err := core.database.GetByNameOrEmail(name, email)
	if err != nil {
		return nil, fmt.Errorf("error getting user from database: %w", err)
	}

	return user, nil
}

func (core *User) Update(userID model.ID, partial model.UserPartial) error {
	err := validate(core.validator, partial)
	if err != nil {
		return err
	}

	user, err := core.GetByID(userID)
	if err != nil {
		return fmt.Errorf("error checking if user exist in database: %w", err)
	}

	hash, err := argon2id.CreateHash(partial.Password, &core.argon2id)
	if err != nil {
		return fmt.Errorf("error creating password hash: %w", err)
	}

	user.Password = hash

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error updating user in database: %w", err)
	}

	return nil
}

func (core *User) Delete(userID model.ID, deleteByID model.ID) error {
	user, err := core.GetByID(userID)
	if err != nil {
		return err
	}

	if user.IsProtected {
		return ErrUserIsProtected
	}

	user.DeletedAt = time.Now()
	user.DeletedBy = deleteByID

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error deleting user from database: %w", err)
	}

	return nil
}

func (core *User) IsAdmin(userID model.ID) (bool, error) {
	user, err := core.GetByID(userID)
	if err != nil {
		return false, err
	}

	return user.IsAdmin, nil
}

func (core *User) NewAdmin(userID model.ID) error {
	user, err := core.GetByID(userID)
	if err != nil {
		return err
	}

	user.IsAdmin = true

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error updating user in database: %w", err)
	}

	return nil
}

func (core *User) RemoveAdmin(userID model.ID) error {
	user, err := core.GetByID(userID)
	if err != nil {
		return err
	}

	if user.IsProtected {
		return ErrUserIsProtected
	}

	if !user.IsAdmin {
		return ErrUserIsNotAdmin
	}

	user.IsAdmin = false

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error updating user in database: %w", err)
	}

	return nil
}

func (core *User) Protected(userID model.ID) error {
	user, err := core.GetByID(userID)
	if err != nil {
		return err
	}

	user.IsAdmin = true
	user.IsProtected = true

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error updating user in database: %w", err)
	}

	return nil
}

func (core *User) NewSession(partial model.UserSessionPartial) (*model.UserSession, error) {
	err := validate(core.validator, partial)
	if err != nil {
		return nil, err
	}

	user, err := core.GetByNameOrEmail(partial.Name, partial.Email)
	if err != nil {
		return nil, err
	}

	equals, err := argon2id.ComparePasswordAndHash(partial.Password, user.Password)
	if err != nil {
		return nil, fmt.Errorf("error comparing password with hash: %w", err)
	}

	if !equals {
		return nil, ErrUserWrongPassword
	}

	session := model.UserSession{
		ID:        model.NewID(),
		UserID:    user.ID,
		CreateaAt: time.Now(),
		Expires:   time.Now().Add(core.durationSession),
		DeletedAt: time.Now().Add(core.durationSession),
	}

	err = core.database.SaveSession(session)
	if err != nil {
		return nil, fmt.Errorf("error saving session in database: %w", err)
	}

	return &session, nil
}

func (core *User) GetSession(sessionID model.ID) (*model.UserSession, error) {
	exist, err := core.database.ExistSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("error checking if session exist in database: %w", err)
	}

	if !exist {
		return nil, ErrUserSessionDoesNotExist
	}

	session, err := core.database.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("error getting session from database: %w", err)
	}

	exist, err = core.existByID(session.UserID)
	if err != nil {
		return nil, fmt.Errorf("error checking if user exist in database: %w", err)
	}

	if !exist {
		return nil, ErrUserSessionDoesNotExist
	}

	if session.DeletedAt.Before(time.Now()) {
		return nil, ErrUserSessionDeleted
	}

	return session, nil
}

func (core *User) ReplaceSession(sessionID model.ID) (*model.UserSession, error) {
	currentSession, err := core.GetSession(sessionID)
	if err != nil && !errors.Is(err, ErrUserSessionDeleted) {
		return nil, err
	}

	if currentSession.DeletedAt.IsZero() {
		currentSession.DeletedAt = time.Now()

		err = core.database.UpdateSession(*currentSession)
		if err != nil {
			return nil, fmt.Errorf("error updating session in database: %w", err)
		}
	}

	newSession := model.UserSession{
		ID:        model.NewID(),
		UserID:    currentSession.UserID,
		CreateaAt: time.Now(),
		Expires:   time.Now().Add(core.durationSession),
		DeletedAt: time.Now().Add(core.durationSession),
	}

	err = core.database.SaveSession(newSession)
	if err != nil {
		return nil, fmt.Errorf("error saving session in database: %w", err)
	}

	return &newSession, nil
}

func newUser(
	database *data.User,
	validate *validator.Validate,
	durationSession time.Duration,
) *User {
	return &User{
		database:  database,
		validator: validate,
		argon2id: argon2id.Params{
			Memory:      argon2idParamMemory,
			Iterations:  argon2idParamIterations,
			Parallelism: argon2idParamParallelism,
			SaltLength:  argon2idParamSaltLength,
			KeyLength:   argon2idParamKeyLength,
		},
		durationSession: durationSession,
	}
}
