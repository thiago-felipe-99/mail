package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/thiago-felipe-99/mail/publisher/model"
)

type rabbitConfig struct {
	User     string `config:"user"     validate:"required"`
	Password string `config:"password" validate:"required"`
	Host     string `config:"host"     validate:"required"`
	Port     int    `config:"port"     validate:"required"`
	Vhost    string `config:"vhost"    validate:"required"`
}

type minioConfig struct {
	Host           string `config:"host"            validate:"required"`
	Port           int    `config:"port"            validate:"required"`
	AccessKey      string `config:"access_key"      validate:"required"`
	SecretKey      string `config:"secret_key"      validate:"required"`
	Secure         bool   `config:"secure"`
	TemplateBucket string `config:"template_bucket" validate:"required"`
}

type mongoConfig struct {
	User             string `config:"user"               validate:"required"`
	Password         string `config:"password"           validate:"required"`
	Host             string `config:"host"               validate:"required"`
	Port             int    `config:"port"               validate:"required"`
	ConnectTimeoutMS int    `config:"connect_timeout_ms" validate:"required"`
	TimeoutMS        int    `config:"timeout_ms"         validate:"required"`
	MaxIdleTimeMS    int    `config:"max_idle_time_ms"   validate:"required"`
	Secure           bool   `config:"secure"`
}

type sessionConfig struct {
	DurationMinutes int `config:"duration_minutes" validate:"required,min=1"`
}

type adminConfig = model.UserPartial

type configurations struct {
	Rabbit  rabbitConfig  `config:"rabbit"  validate:"required"`
	Minio   minioConfig   `config:"minio"   validate:"required"`
	Mongo   mongoConfig   `config:"mongo"   validate:"required"`
	Session sessionConfig `config:"session" validate:"required"`
	Admin   adminConfig   `config:"admin"   validate:"required"`
}

//nolint:gomnd
func defaultConfigurations() configurations {
	return configurations{
		Rabbit: rabbitConfig{
			Port:  5672,
			Vhost: "/",
		},
		Minio: minioConfig{
			Port:   9000,
			Secure: true,
		},
		Mongo: mongoConfig{
			ConnectTimeoutMS: 10000,
			TimeoutMS:        5000,
			MaxIdleTimeMS:    100,
			Secure:           true,
		},
		Session: sessionConfig{
			DurationMinutes: 5,
		},
	}
}

func parseEnv(env string) string {
	keys := strings.SplitN(env, "_", 2) //nolint:gomnd
	size := len(keys)

	var key string

	switch size {
	case 0:
		return ""
	case 1:
		key = keys[0]
	default:
		key = keys[0] + "__" + strings.Join(keys[1:], "_")
	}

	return strings.ToLower(key)
}

func getConfigurations() (*configurations, error) {
	koanfConfig := koanf.Conf{
		Delim:       "__",
		StrictMerge: false,
	}

	configRaw := koanf.NewWithConf(koanfConfig)

	err := configRaw.Load(structs.Provider(defaultConfigurations(), "config"), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get default configurations: %w", err)
	}

	err = configRaw.Load(file.Provider(".env"), dotenv.ParserEnv("", "__", parseEnv))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("unable to get .env file: %w", err)
	}

	err = configRaw.Load(env.Provider("", "__", parseEnv), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get environment vaiables: %w", err)
	}

	config := &configurations{}

	err = configRaw.UnmarshalWithConf("", config, koanf.UnmarshalConf{
		Tag:       "config",
		FlatPaths: false,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal configurations: %w", err)
	}

	validate := validator.New()

	err = validate.Struct(config)
	if err != nil {
		return nil, fmt.Errorf("error validating configurations: %w", err)
	}

	return config, nil
}
