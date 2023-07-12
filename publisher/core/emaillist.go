package core

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/thiago-felipe-99/mail/publisher/data"
	"github.com/thiago-felipe-99/mail/publisher/model"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type EmailList struct {
	database  *data.EmailList
	validator *validator.Validate
}

func uniq[T comparable](data []T) []T {
	uniqs := make([]T, 0, len(data))

	for _, element := range data {
		if !slices.Contains(uniqs, element) {
			uniqs = append(uniqs, element)
		}
	}

	return uniqs
}

func (core *EmailList) Create(userID model.ID, partial model.EmailListPartial) error {
	err := validate(core.validator, partial)
	if err != nil {
		return err
	}

	exist, err := core.database.ExistByName(partial.Name, userID)
	if err != nil {
		return fmt.Errorf("error checking if email list exist in database: %w", err)
	}

	if exist {
		return ErrEmailListAlreadyExist
	}

	uniqEmails := uniq(partial.Emails)

	emails := make(map[model.ID]string, len(uniqEmails))
	for _, email := range uniqEmails {
		emails[model.NewID()] = email
	}

	list := model.EmailList{
		ID:          model.NewID(),
		Emails:      emails,
		Name:        partial.Name,
		EmailAlias:  partial.EmailAlias,
		Description: partial.Description,
		CreatedAt:   time.Now(),
		CreatedBy:   userID,
		DeletedAt:   time.Time{},
		DeletedBy:   model.ID{},
	}

	err = core.database.Create(list)
	if err != nil {
		return fmt.Errorf("error creating email list in database: %w", err)
	}

	return nil
}

func (core *EmailList) GetAll(userID model.ID) ([]model.EmailList, error) {
	emailList, err := core.database.GetAllUser(userID)
	if err != nil {
		return nil, fmt.Errorf("error getting email list: %w", err)
	}

	return emailList, nil
}

func (core *EmailList) Get(name string, userID model.ID) (*model.EmailList, error) {
	exist, err := core.database.ExistByName(name, userID)
	if err != nil {
		return nil, fmt.Errorf("error checking if email list exist in database: %w", err)
	}

	if !exist {
		return nil, ErrEmailListDoesNotExist
	}

	emailList, err := core.database.GetByName(name, userID)
	if err != nil {
		return nil, fmt.Errorf("error getting email list: %w", err)
	}

	return emailList, nil
}

func (core *EmailList) UpdateInfo(name string, userID model.ID, info model.EmailListInfo) error {
	err := validate(core.validator, info)
	if err != nil {
		return err
	}

	emailList, err := core.Get(name, userID)
	if err != nil {
		return err
	}

	if emailList.Name != info.Name {
		exist, err := core.database.ExistByName(info.Name, userID)
		if err != nil {
			return fmt.Errorf("error checking if email list exist in database: %w", err)
		}

		if exist {
			return ErrEmailListAlreadyExist
		}
	}

	err = core.database.UpdateInfo(emailList.ID, info)
	if err != nil {
		return fmt.Errorf("error updating email list info: %w", err)
	}

	return nil
}

func (core *EmailList) Delete(name string, userID model.ID, deletedBy model.ID) error {
	emailList, err := core.Get(name, userID)
	if err != nil {
		return err
	}

	emailList.DeletedAt = time.Now()
	emailList.DeletedBy = deletedBy

	err = core.database.Update(*emailList)
	if err != nil {
		return fmt.Errorf("error deleting email list: %w", err)
	}

	return nil
}

func (core *EmailList) AddEmails(name string, userID model.ID, emails model.EmailListEmails) error {
	err := validate(core.validator, emails)
	if err != nil {
		return err
	}

	emailList, err := core.Get(name, userID)
	if err != nil {
		return err
	}

	emailsAlreadyExist := maps.Values(emailList.Emails)

	for _, email := range emails.Emails {
		if !slices.Contains(emailsAlreadyExist, email) {
			emailList.Emails[model.NewID()] = email
		}
	}

	err = core.database.Update(*emailList)
	if err != nil {
		return fmt.Errorf("error adding emails: %w", err)
	}

	return nil
}

func (core *EmailList) RemoveEmails(
	name string,
	userID model.ID,
	emails model.EmailListEmails,
) error {
	err := validate(core.validator, emails)
	if err != nil {
		return err
	}

	emailList, err := core.Get(name, userID)
	if err != nil {
		return err
	}

	emailsAlreadyExist := maps.Values(emailList.Emails)

	del := func(k model.ID, v string) bool {
		return slices.Contains(emailsAlreadyExist, v)
	}

	maps.DeleteFunc(emailList.Emails, del)

	err = core.database.Update(*emailList)
	if err != nil {
		return fmt.Errorf("error removing emails: %w", err)
	}

	return nil
}

func (core *EmailList) RemoveEmail(name string, userID model.ID, email model.ID) error {
	emailList, err := core.Get(name, userID)
	if err != nil {
		return err
	}

	del := func(k model.ID, v string) bool {
		return k == email
	}

	maps.DeleteFunc(emailList.Emails, del)

	err = core.database.Update(*emailList)
	if err != nil {
		return fmt.Errorf("error removing email: %w", err)
	}

	return nil
}

func newEmailList(
	database *data.EmailList,
	validate *validator.Validate,
) *EmailList {
	return &EmailList{
		database:  database,
		validator: validate,
	}
}
