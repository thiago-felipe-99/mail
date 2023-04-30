//nolint:wrapcheck
package controllers

import (
	"errors"
	"log"
	"time"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/pt"
	"github.com/go-playground/locales/pt_BR"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	ptTranslations "github.com/go-playground/validator/v10/translations/pt"
	pt_br_translations "github.com/go-playground/validator/v10/translations/pt_BR"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
	"github.com/minio/minio-go/v7"
	"github.com/thiago-felipe-99/mail/publisher/core"
	"github.com/thiago-felipe-99/mail/publisher/data"
	"github.com/thiago-felipe-99/mail/rabbit"
)

type sent struct {
	Message string `json:"message" bson:"message"`
}

type expectError struct {
	err    error
	status int
}

type okay struct {
	message string
	status  int
}

func callingCore(
	coreFunc func() error,
	expectErrors []expectError,
	unexpectMessageError string,
	okay okay,
	language ut.Translator,
	handler *fiber.Ctx,
) error {
	err := coreFunc()
	if err != nil {
		modelInvalid := core.ModelInvalidError{}
		if okay := errors.As(err, &modelInvalid); okay {
			return handler.Status(fiber.StatusBadRequest).
				JSON(sent{modelInvalid.Translate(language)})
		}

		for _, expectError := range expectErrors {
			if errors.Is(err, expectError.err) {
				return handler.Status(expectError.status).JSON(sent{expectError.err.Error()})
			}
		}

		log.Printf("[ERROR] - %s: %s", unexpectMessageError, err)

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{unexpectMessageError})
	}

	return handler.Status(okay.status).JSON(sent{okay.message})
}

func callingCoreWithReturn[T any](
	coreFunc func() (T, error),
	expectErrors []expectError,
	unexpectMessageError string,
	handler *fiber.Ctx,
) error {
	data, err := coreFunc()
	if err != nil {
		for _, expectError := range expectErrors {
			if errors.Is(err, expectError.err) {
				return handler.Status(expectError.status).JSON(sent{expectError.err.Error()})
			}
		}

		log.Printf("[ERROR] - %s: %s", unexpectMessageError, err)

		return handler.Status(fiber.StatusInternalServerError).
			JSON(sent{unexpectMessageError})
	}

	return handler.JSON(data)
}

func createTranslator(validate *validator.Validate) (*ut.UniversalTranslator, error) {
	translator := ut.New(en.New(), pt.New(), pt_BR.New())

	enTrans, _ := translator.GetTranslator("en")

	err := en_translations.RegisterDefaultTranslations(validate, enTrans)
	if err != nil {
		return nil, err
	}

	ptTrans, _ := translator.GetTranslator("pt")

	err = ptTranslations.RegisterDefaultTranslations(validate, ptTrans)
	if err != nil {
		return nil, err
	}

	ptBRTrans, _ := translator.GetTranslator("pt_BR")

	err = pt_br_translations.RegisterDefaultTranslations(validate, ptBRTrans)
	if err != nil {
		return nil, err
	}

	return translator, nil
}

func CreateHTTPServer(
	rabbit *rabbit.Rabbit,
	userDatabase *data.User,
	queueDatabase *data.Queue,
	templateDatabase *data.Template,
	minio *minio.Client,
	bucket string,
) (*fiber.App, error) {
	app := fiber.New()

	prometheus := fiberprometheus.New("publisher")
	prometheus.RegisterAt(app, "/metrics")

	app.Use(logger.New(logger.Config{
		//nolint:lll
		Format:     "${time} [INFO] - Finished request | ${ip} | ${status} | ${latency} | ${method} | ${path} | ${bytesSent} | ${bytesReceived} | ${error}\n",
		TimeFormat: "2006/01/02 15:04:05",
	}))
	app.Use(recover.New())
	app.Use(prometheus.Middleware)

	swaggerConfig := swagger.Config{
		Title:                  "Emails Publisher",
		WithCredentials:        true,
		DisplayRequestDuration: true,
	}

	app.Get("/swagger/*", swagger.New(swaggerConfig))

	validate := validator.New()

	translator, err := createTranslator(validate)
	if err != nil {
		return nil, err
	}

	languages := []string{"en", "pt_BR", "pt"}

	user := User{
		core:       core.NewUser(userDatabase, validate, time.Minute*5), //nolint:gomnd
		translator: translator,
		languages:  languages,
	}

	template := Template{
		core:       core.NewTemplate(templateDatabase, minio, bucket, validate),
		translator: translator,
		languages:  languages,
	}

	queue := Queue{
		core:       core.NewQueue(template.core, rabbit, queueDatabase, validate),
		translator: translator,
		languages:  languages,
	}

	app.Post("/user", user.create)
	app.Post("/user/session", user.newSession)

	app.Get("/email/queue", queue.getAll)
	app.Post("/email/queue", queue.create)
	app.Delete("/email/queue/:name", queue.delete)
	app.Post("/email/queue/:name/send", queue.sendEmail)

	app.Get("/email/template", template.getAll)
	app.Post("/email/template", template.create)
	app.Get("/email/template/:name", template.get)
	app.Put("/email/template/:name", template.update)
	app.Delete("/email/template/:name", template.delete)

	return app, nil
}