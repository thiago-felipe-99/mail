package controllers

import (
	"errors"
	"fmt"
	"log"

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
	"github.com/thiago-felipe-99/mail/publisher/core"
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
		return nil, fmt.Errorf("error register 'en' translation: %w", err)
	}

	ptTrans, _ := translator.GetTranslator("pt")

	err = ptTranslations.RegisterDefaultTranslations(validate, ptTrans)
	if err != nil {
		return nil, fmt.Errorf("error register 'pt' translation: %w", err)
	}

	ptBRTrans, _ := translator.GetTranslator("pt_BR")

	err = pt_br_translations.RegisterDefaultTranslations(validate, ptBRTrans)
	if err != nil {
		return nil, fmt.Errorf("error register 'pt_BR' translation: %w", err)
	}

	return translator, nil
}

func CreateHTTPServer(validate *validator.Validate, cores *core.Cores) (*fiber.App, error) {
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

	translator, err := createTranslator(validate)
	if err != nil {
		return nil, err
	}

	languages := []string{"en", "pt_BR", "pt"}

	user := User{
		core:       cores.User,
		translator: translator,
		languages:  languages,
	}

	template := Template{
		core:       cores.Template,
		translator: translator,
		languages:  languages,
	}

	queue := Queue{
		core:       cores.Queue,
		translator: translator,
		languages:  languages,
	}

	app.Post("/user/session", user.newSession)

	app.Use(user.refreshSession)

	app.Get("/user", user.get)
	app.Post("/user", user.isAdmin, user.create)
	app.Put("/user", user.update)
	app.Delete("/user", user.delete)

	app.Put("/user/session", func(c *fiber.Ctx) error { return c.JSON(sent{"session refreshed"}) })

	app.Get("/user/admin/:userID", user.isAdmin, user.getByAdmin)
	app.Post("/user/admin/:userID", user.isAdmin, user.newAdmin)
	app.Delete("/user/admin/:userID", user.isAdmin, user.removeAdminRole)
	app.Delete("/user/admin/:userID/user", user.isAdmin, user.deleteUserAdmin)
	app.Get("/user/all", user.isAdmin, user.getAll)

	app.Get("/user/role", user.getRoles)
	app.Post("/user/role", user.isAdmin, user.createRole)
	app.Delete("/user/role", user.deleteRolesByCurrentUser)
	app.Put("/user/role/:userID", user.hasRoles, user.addRoles)
	app.Delete("/user/role/:userID", user.hasRolesAdmin, user.deleteRolesByRolesAdmin)
	app.Delete("/user/role/:userID/admin", user.isAdmin, user.deleteRolesByAdmin)

	app.Get("/email/queue", queue.getAll)
	app.Post("/email/queue", user.isAdmin, queue.create)
	app.Delete("/email/queue/:name", user.isAdmin, queue.delete)
	app.Post("/email/queue/:name/send", queue.sendEmail)

	// app.Get("/email/template", template.getByUser)
	app.Post("/email/template", template.create)
	app.Get("/email/template/all", user.isAdmin, template.getAll)
	app.Get("/email/template/:name", template.get)
	app.Put("/email/template/:name", template.update)
	app.Delete("/email/template/:name", template.delete)

	return app, nil
}
