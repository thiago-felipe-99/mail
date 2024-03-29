package core

import (
	"context"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/thiago-felipe-99/mail/publisher/data"
	"github.com/thiago-felipe-99/mail/publisher/model"
	"github.com/thiago-felipe-99/mail/rabbit"
)

type Queue struct {
	template   *Template
	attachment *Attachment
	emailList  *EmailList
	rabbit     *rabbit.Rabbit
	database   *data.Queue
	validator  *validator.Validate
}

func (core *Queue) Exist(name string) (bool, error) {
	exist, err := core.database.Exist(name)
	if err != nil {
		return false, fmt.Errorf("error checking if queue exist in database: %w", err)
	}

	return exist, nil
}

func (core *Queue) Create(partial model.QueuePartial, userID model.ID) error {
	err := validate(core.validator, partial)
	if err != nil {
		return err
	}

	queue := model.Queue{
		ID:         model.NewID(),
		Name:       partial.Name,
		DLX:        partial.Name + "-dlx",
		MaxRetries: partial.MaxRetries,
		CreatedAt:  time.Now(),
		CreatedBy:  userID,
		DeletedAt:  time.Time{},
		DeletedBy:  model.ID{},
	}

	queueExist, err := core.Exist(queue.Name)
	if err != nil {
		return fmt.Errorf("error checking if queue exist in database: %w", err)
	}

	dlxExist, err := core.Exist(queue.Name)
	if err != nil {
		return fmt.Errorf("error checking if dlx queue exist in database: %w", err)
	}

	if queueExist || dlxExist {
		return ErrQueueAlreadyExist
	}

	err = core.rabbit.CreateQueueWithDLX(queue.Name, queue.DLX, queue.MaxRetries)
	if err != nil {
		return fmt.Errorf("error creating queue: %w", err)
	}

	err = core.database.Create(queue)
	if err != nil {
		return fmt.Errorf("error creating queue in database: %w", err)
	}

	return nil
}

func (core *Queue) Get(name string) (*model.Queue, error) {
	exist, err := core.Exist(name)
	if err != nil {
		return nil, fmt.Errorf("error checking if queue exist: %w", err)
	}

	if !exist {
		return nil, ErrQueueDoesNotExist
	}

	queue, err := core.database.Get(name)
	if err != nil {
		return nil, fmt.Errorf("error getting queue from database: %w", err)
	}

	return queue, nil
}

func (core *Queue) GetAll() ([]model.Queue, error) {
	queues, err := core.database.GetAll()
	if err != nil {
		return nil, fmt.Errorf("error getting all queues: %w", err)
	}

	return queues, nil
}

func (core *Queue) Delete(name string, userID model.ID) error {
	if len(name) == 0 {
		return ErrInvalidName
	}

	queue, err := core.Get(name)
	if err != nil {
		return err
	}

	err = core.rabbit.DeleteQueueWithDLX(queue.Name, queue.DLX)
	if err != nil {
		return fmt.Errorf("error deleting queue from RabbitMQ: %w", err)
	}

	queue.DeletedAt = time.Now()
	queue.DeletedBy = userID

	err = core.database.Update(*queue)
	if err != nil {
		return fmt.Errorf("error deleting queue from database: %w", err)
	}

	return nil
}

func (core *Queue) SendEmail(queue string, partial model.EmailPartial, userID model.ID) error {
	if len(queue) == 0 {
		return ErrInvalidName
	}

	err := validate(core.validator, partial)
	if err != nil {
		return err
	}

	queueExist, err := core.Exist(queue)
	if err != nil {
		return fmt.Errorf("error checking if queue exist: %w", err)
	}

	if !queueExist {
		return ErrQueueDoesNotExist
	}

	if partial.Template != nil {
		exist, err := core.template.Exist(partial.Template.Name)
		if err != nil {
			return fmt.Errorf("error checking if template exist: %w", err)
		}

		if !exist {
			return ErrTemplateDoesNotExist
		}

		fields, err := core.template.GetFields(partial.Template.Name)
		if err != nil {
			return fmt.Errorf("error getting templates fields: %w", err)
		}

		for _, field := range fields {
			if _, found := partial.Template.Data[field]; !found {
				return ErrMissingFieldTemplates
			}
		}
	}

	for _, list := range partial.EmailLists {
		emailList, err := core.emailList.Get(list, userID)
		if err != nil {
			return err
		}

		partial.Receivers = append(partial.Receivers, model.Receiver{
			Name:  emailList.Name,
			Email: emailList.EmailAlias,
		})

		blindReceiver := make([]model.Receiver, 0, len(emailList.Emails))
		for _, email := range emailList.Emails {
			blindReceiver = append(blindReceiver, model.Receiver{
				Name:  emailList.Name,
				Email: email,
			})
		}

		partial.BlindReceivers = append(partial.BlindReceivers, blindReceiver...)
	}

	for _, attachment := range partial.Attachments {
		uploaded, err := core.attachment.Uploaded(userID, attachment)
		if err != nil {
			return fmt.Errorf("error checking if attachment exist: %w", err)
		}

		if !uploaded {
			return ErrAttachmentDoesNotExist
		}
	}

	err = core.rabbit.SendMessage(context.Background(), queue, partial)
	if err != nil {
		return fmt.Errorf("error sending email: %w", err)
	}

	email := model.Email{
		ID:             model.NewID(),
		UserID:         userID,
		EmailLists:     partial.EmailLists,
		Receivers:      partial.Receivers,
		BlindReceivers: partial.BlindReceivers,
		Subject:        partial.Subject,
		Message:        partial.Message,
		Template:       partial.Template,
		Attachments:    partial.Attachments,
		SentAt:         time.Now(),
	}

	err = core.database.SaveEmail(email)
	if err != nil {
		return fmt.Errorf("error saving email in database: %w", err)
	}

	return nil
}

func newQueue(
	template *Template,
	attachment *Attachment,
	emailList *EmailList,
	rabbit *rabbit.Rabbit,
	database *data.Queue,
	validate *validator.Validate,
) *Queue {
	return &Queue{
		template:   template,
		attachment: attachment,
		emailList:  emailList,
		rabbit:     rabbit,
		database:   database,
		validator:  validate,
	}
}
