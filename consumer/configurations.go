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
)

type sender struct {
	Name  string `config:"name"  validate:"required"`
	Email string `config:"email" validate:"required"`
}

type smtp struct {
	User     string `config:"user"     validate:"required"`
	Password string `config:"password" validate:"required"`
	Host     string `config:"host"     validate:"required"`
	Port     int    `config:"port"     validate:"required"`
}

type rabbitConfig struct {
	User       string `config:"user"        validate:"required"`
	Password   string `config:"password"    validate:"required"`
	Host       string `config:"host"        validate:"required"`
	Port       int    `config:"port"        validate:"required"`
	Vhost      string `config:"vhost"       validate:"required"`
	Queue      string `config:"queue"       validate:"required"`
	QueueDLX   string `config:"queue_dlx"   validate:"required"`
	MaxRetries int64  `config:"max_retries" validate:"required"`
}

type buffer struct {
	Size     int `config:"size"     validate:"required"`
	Quantity int `config:"quantity" validate:"required"`
}

type cacheConfig struct {
	Bucket       string `config:"bucket"         validate:"required"`
	Shards       int    `config:"shards"         validate:"required"`
	LifeWindow   int    `config:"life_window"    validate:"required"`
	CleanWindow  int    `config:"clean_window"   validate:"required"`
	AvgEntries   int    `config:"avg_entries"    validate:"required"`
	AvgEntrySize int    `config:"avg_entry_size" validate:"required"`
	MaxEntrySize int    `config:"max_entry_size" validate:"required"`
	MaxSize      int    `config:"max_size"       validate:"required"`
	Statics      bool   `config:"statics"`
	Verbose      bool   `config:"verbose"`
}

type minioConfig struct {
	Host      string `config:"host"       validate:"required"`
	Port      int    `config:"port"       validate:"required"`
	AccessKey string `config:"access_key" validate:"required"`
	SecretKey string `config:"secret_key" validate:"required"`
	Secure    bool   `config:"secure"`
}

type configurations struct {
	Sender   sender       `config:"sender"   validate:"required"`
	SMTP     smtp         `config:"smtp"     validate:"required"`
	Rabbit   rabbitConfig `config:"rabbit"   validate:"required"`
	Buffer   buffer       `config:"buffer"   validate:"required"`
	Timeout  int          `config:"timeout"  validate:"required"`
	Cache    cacheConfig  `config:"cache"    validate:"required"`
	Template cacheConfig  `config:"template" validate:"required"`
	Minio    minioConfig  `config:"minio"    validate:"required"`
}

//nolint:gomnd
func defaultConfigurations() configurations {
	return configurations{
		SMTP: smtp{
			Port: 587,
		},
		Rabbit: rabbitConfig{
			Port:       5672,
			Vhost:      "/",
			MaxRetries: 4,
		},
		Buffer: buffer{
			Size:     100,
			Quantity: 10,
		},
		Cache: cacheConfig{
			Shards:       64,
			LifeWindow:   60,
			CleanWindow:  5,
			AvgEntries:   10,
			AvgEntrySize: 10,
			MaxEntrySize: 25,
			MaxSize:      1000,
			Statics:      false,
			Verbose:      false,
		},
		Template: cacheConfig{
			Shards:       64,
			AvgEntries:   10,
			AvgEntrySize: 1,
			MaxEntrySize: 2,
			MaxSize:      100,
			Statics:      false,
			Verbose:      false,
		},
		Minio: minioConfig{
			Port:   9000,
			Secure: true,
		},
		Timeout: 2,
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
		validationErrs := validator.ValidationErrors{}

		okay := errors.As(err, &validationErrs)
		if !okay {
			return nil, fmt.Errorf("error on validating configurations: %w", err)
		}

		for index := len(validationErrs) - 1; index >= 0; index-- {
			name := validationErrs[index].Namespace()
			if name == "configurations.Template.CleanWindow" ||
				name == "configurations.Template.LifeWindow" {
				validationErrs[index] = validationErrs[len(validationErrs)-1]
				validationErrs = validationErrs[:len(validationErrs)-1]
			}
		}

		if len(validationErrs) > 0 {
			return nil, fmt.Errorf("error on validating configurations: %w", validationErrs)
		}

		return config, nil
	}

	return config, nil
}
