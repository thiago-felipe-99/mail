package main

import (
	"fmt"
	"log"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/thiago-felipe-99/mail/publisher/controllers"
	"github.com/thiago-felipe-99/mail/publisher/core"
	"github.com/thiago-felipe-99/mail/publisher/data"
	_ "github.com/thiago-felipe-99/mail/publisher/docs"
	"github.com/thiago-felipe-99/mail/rabbit"
)

// @title			Publisher Emails
// @version		1.0
// @host			localhost:8080
// @BasePath		/
// @description	This is an api that publishes emails in RabbitMQ.
func main() {
	configs, err := getConfigurations()
	if err != nil {
		log.Printf("[ERROR] - Error getting configurations: %s", err)

		return
	}

	rabbitConfig := rabbit.Config{
		User:     configs.Rabbit.User,
		Password: configs.Rabbit.Password,
		Host:     configs.Rabbit.Host,
		Port:     fmt.Sprint(configs.Rabbit.Port),
		Vhost:    configs.Rabbit.Vhost,
	}

	rabbitConnection := rabbit.New(rabbitConfig)
	defer rabbitConnection.Close()

	go rabbitConnection.HandleConnection()

	minioURI := fmt.Sprintf("%s:%d", configs.Minio.Host, configs.Minio.Port)

	minio, err := minio.New(minioURI, &minio.Options{
		Creds: credentials.NewStaticV4(configs.Minio.AccessKey, configs.Minio.SecretKey, ""),
	})
	if err != nil {
		log.Printf("[ERROR] - Error connecting with the Minio: %s", err)

		return
	}

	var mongoURI string

	if configs.Mongo.Secure {
		mongoURI = fmt.Sprintf(
			"mongodb+srv://%s:%s@%s:%d/?connectTimeoutMS=%d&timeoutMS=%d&maxIdleTimeMS=%d",
			configs.Mongo.User,
			configs.Mongo.Password,
			configs.Mongo.Host,
			configs.Mongo.Port,
			configs.Mongo.ConnectTimeoutMS,
			configs.Mongo.TimeoutMS,
			configs.Mongo.MaxIdleTimeMS,
		)
	} else {
		mongoURI = fmt.Sprintf(
			"mongodb://%s:%s@%s:%d/?connectTimeoutMS=%d&timeoutMS=%d&maxIdleTimeMS=%d",
			configs.Mongo.User,
			configs.Mongo.Password,
			configs.Mongo.Host,
			configs.Mongo.Port,
			configs.Mongo.ConnectTimeoutMS,
			configs.Mongo.TimeoutMS,
			configs.Mongo.MaxIdleTimeMS,
		)
	}

	mongoClient, err := data.NewMongoClient(mongoURI)
	if err != nil {
		log.Printf("[ERROR] - Error creating datase: %s", err)

		return
	}

	databases := data.NewDatabases(mongoClient)

	queues, err := databases.Queue.GetAll()
	if err != nil {
		log.Printf("[ERROR] - Error getting queues: %s", err)

		return
	}

	for _, queue := range queues {
		err := rabbitConnection.CreateQueueWithDLX(queue.Name, queue.DLX, queue.MaxRetries)
		if err != nil {
			log.Printf("[ERROR] - Error creating queue: %s", err)

			return
		}
	}

	validate := validator.New()

	cores := core.NewCores(
		databases,
		validate,
		time.Duration(configs.Session.DurationMinutes)*time.Minute,
		rabbitConnection,
		minio,
		configs.Minio.TemplateBucket,
	)

	exist, err := cores.User.ExistByNameOrEmail(configs.Admin.Name, configs.Admin.Email)
	if err != nil {
		log.Printf("[ERROR] - Error verifying if first user exist: %s", err)
	}

	if !exist {
		err := cores.User.Create(configs.Admin, uuid.UUID{})
		if err != nil {
			log.Printf("[ERROR] - Error creating first user: %s", err)

			return
		}

		user, err := cores.User.GetByNameOrEmail(configs.Admin.Name, configs.Admin.Email)
		if err != nil {
			log.Printf("[ERROR] - Error getting first user: %s", err)

			return
		}

		err = cores.User.Protected(user.ID)
		if err != nil {
			log.Printf("[ERROR] - Error creating first admin: %s", err)

			return
		}
	}

	server, err := controllers.CreateHTTPServer(validate, cores)
	if err != nil {
		log.Printf("[ERROR] - Error create server: %s", err)

		return
	}

	err = server.Listen(":8080")
	if err != nil {
		log.Printf("[ERROR] - Error listen HTTP server: %s", err)

		return
	}
}
