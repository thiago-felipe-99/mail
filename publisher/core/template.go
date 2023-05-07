package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/thiago-felipe-99/mail/publisher/data"
	"github.com/thiago-felipe-99/mail/publisher/model"
)

type Template struct {
	minio       *minio.Client
	bucket      string
	database    *data.Template
	validate    *validator.Validate
	regexFields *regexp.Regexp
}

func (core *Template) getFields(template string) []string {
	fieldsRaw := core.regexFields.FindAllString(template, -1)

	fields := make([]string, 0, len(fieldsRaw))

	existField := func(fields []string, find string) bool {
		for _, field := range fields {
			if field == find {
				return true
			}
		}

		return false
	}

	for _, field := range fieldsRaw {
		field = strings.Trim(field, "{} ")

		if !existField(fields, field) {
			fields = append(fields, field)
		}
	}

	return fields
}

func (core *Template) Exist(name string) (bool, error) {
	exist, err := core.database.Exist(name)
	if err != nil {
		return false, fmt.Errorf("error checking if template exist in database: %w", err)
	}

	return exist, nil
}

func (core *Template) Create(partial model.TemplatePartial, userID uuid.UUID) error {
	err := validate(core.validate, partial)
	if err != nil {
		return err
	}

	if len(partial.Template) > maxSizeTemplate {
		return ErrMaxSizeTemplate
	}

	exist, err := core.Exist(partial.Name)
	if err != nil {
		return fmt.Errorf("error checking if template exist: %w", err)
	}

	if exist {
		return ErrTemplateNameAlreadyExist
	}

	template := model.Template{
		ID:        uuid.New(),
		Name:      partial.Name,
		Template:  partial.Template,
		Fields:    core.getFields(partial.Template),
		Roles:     []string{},
		CreatedAt: time.Now(),
		CreatedBy: userID,
		DeletedAt: time.Time{},
		DeletedBy: uuid.UUID{},
	}

	templateReader := strings.NewReader(template.Template)

	_, err = core.minio.PutObject(
		context.Background(),
		core.bucket,
		template.Name,
		templateReader,
		templateReader.Size(),
		minio.PutObjectOptions{
			ContentType: "text/markdown",
		},
	)
	if err != nil {
		return fmt.Errorf("error creating template in Minio: %w", err)
	}

	err = core.database.Create(template)
	if err != nil {
		return fmt.Errorf("error creating template in database: %w", err)
	}

	return nil
}

func (core *Template) GetAll() ([]model.Template, error) {
	templates, err := core.database.GetAll()
	if err != nil {
		return nil, fmt.Errorf("error getting templates from database: %w", err)
	}

	return templates, nil
}

func (core *Template) Get(name string) (*model.Template, error) {
	if len(name) == 0 {
		return nil, ErrInvalidName
	}

	exist, err := core.Exist(name)
	if err != nil {
		return nil, fmt.Errorf("error checking if template exist: %w", err)
	}

	if !exist {
		return nil, ErrTemplateDoesNotExist
	}

	template, err := core.database.Get(name)
	if err != nil {
		return nil, fmt.Errorf("error getting template from database: %w", err)
	}

	return template, nil
}

func (core *Template) GetFields(name string) ([]string, error) {
	template, err := core.Get(name)
	if err != nil {
		return nil, err
	}

	return template.Fields, nil
}

func (core *Template) Update(name string, partial model.TemplatePartial) error {
	err := validate(core.validate, partial)
	if err != nil {
		return err
	}

	if len(partial.Template) > maxSizeTemplate {
		return ErrMaxSizeTemplate
	}

	template, err := core.Get(name)
	if err != nil {
		return fmt.Errorf("error getting template: %w", err)
	}

	template.Template = partial.Template
	template.Fields = core.getFields(partial.Template)

	templateReader := strings.NewReader(template.Template)

	_, err = core.minio.PutObject(
		context.Background(),
		core.bucket,
		template.Name,
		templateReader,
		templateReader.Size(),
		minio.PutObjectOptions{
			ContentType: "text/markdown",
		},
	)
	if err != nil {
		return fmt.Errorf("error updating template in Minio: %w", err)
	}

	err = core.database.Update(*template)
	if err != nil {
		return fmt.Errorf("error updating template in database: %w", err)
	}

	return nil
}

func (core *Template) Delete(name string, userID uuid.UUID) error {
	if len(name) == 0 {
		return ErrInvalidName
	}

	template, err := core.Get(name)
	if err != nil {
		return err
	}

	err = core.minio.RemoveObject(
		context.Background(),
		core.bucket,
		template.Name,
		minio.RemoveObjectOptions{},
	)
	if err != nil {
		return fmt.Errorf("error deleting template from Minio: %w", err)
	}

	template.DeletedAt = time.Now()
	template.DeletedBy = userID

	err = core.database.Update(*template)
	if err != nil {
		return fmt.Errorf("error deleting template from database: %w", err)
	}

	return nil
}

func NewTemplate(
	database *data.Template,
	minio *minio.Client,
	bucket string,
	validate *validator.Validate,
) *Template {
	return &Template{
		database:    database,
		minio:       minio,
		bucket:      bucket,
		validate:    validate,
		regexFields: regexp.MustCompile(`{{ *(\w|\d)+ *}}`),
	}
}
