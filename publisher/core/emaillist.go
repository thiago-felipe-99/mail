package core

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/thiago-felipe-99/mail/publisher/data"
	"github.com/thiago-felipe-99/mail/publisher/model"
)

type EmailList struct {
	database  *data.EmailList
	validator *validator.Validate
}

func (core *EmailList) Create(userID uuid.UUID, partial model.EmailListPartial) error {
	err := validate(core.validator, partial)
	if err != nil {
		return err
	}

	exist, err := core.database.ExistByName(partial.Name)
	if err != nil {
		return fmt.Errorf("error checking if email list exist in database: %w", err)
	}

	if exist {
		return ErrEmailListAlreadyExist
	}

	list := model.EmailList{
		ID:          uuid.New(),
		Emails:      make(map[uuid.UUID]string, len(partial.Emails)),
		Name:        partial.Name,
		EmailAlias:  partial.EmailAlias,
		Description: partial.Description,
		CreatedAt:   time.Now(),
		CreatedBy:   userID,
		DeletedAt:   time.Time{},
		DeletedBy:   uuid.UUID{},
	}

	for _, email := range partial.Emails {
		list.Emails[uuid.New()] = email
	}

	err = core.database.Create(list)
	if err != nil {
		return fmt.Errorf("error creating email list in database: %w", err)
	}

	return nil
}
