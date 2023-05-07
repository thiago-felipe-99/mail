package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/thiago-felipe-99/mail/publisher/model"
	"go.mongodb.org/mongo-driver/bson"
	mongodb "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongo[T any] struct {
	collection *mongodb.Collection
}

func (database *mongo[T]) create(data T) error {
	_, err := database.collection.InsertOne(context.Background(), data)
	if err != nil {
		return fmt.Errorf("error creating data in database: %w", err)
	}

	return nil
}

func (database *mongo[T]) existByID(id uuid.UUID) (bool, error) {
	filter := bson.D{
		{Key: "_id", Value: id},
		{Key: "deleted_at", Value: bson.D{{Key: "$eq", Value: time.Time{}}}},
	}

	data := new(T)

	err := database.collection.FindOne(context.Background(), filter).Decode(data)
	if err != nil {
		if errors.Is(err, mongodb.ErrNoDocuments) {
			return false, nil
		}

		return false, fmt.Errorf("error getting data from database: %w", err)
	}

	return true, nil
}

func (database *mongo[T]) existByFieldsOr(fields map[string]any) (bool, error) {
	fieldsBson := bson.A{}

	for key, value := range fields {
		fieldsBson = append(fieldsBson, bson.D{{Key: key, Value: value}})
	}

	filter := bson.D{
		{Key: "$or", Value: fieldsBson},
		{Key: "deleted_at", Value: bson.D{{Key: "$eq", Value: time.Time{}}}},
	}

	data := new(T)

	err := database.collection.FindOne(context.Background(), filter).Decode(data)
	if err != nil {
		if errors.Is(err, mongodb.ErrNoDocuments) {
			return false, nil
		}

		return false, fmt.Errorf("error getting data from database: %w", err)
	}

	return true, nil
}

func (database *mongo[T]) getByID(id uuid.UUID) (*T, error) {
	filter := bson.D{
		{Key: "_id", Value: id},
		{Key: "deleted_at", Value: bson.D{{Key: "$eq", Value: time.Time{}}}},
	}

	data := new(T)

	err := database.collection.FindOne(context.Background(), filter).Decode(data)
	if err != nil {
		return nil, fmt.Errorf("error getting data from database: %w", err)
	}

	return data, nil
}

func (database *mongo[T]) getByFieldsOr(fields map[string]any) (*T, error) {
	fieldsBson := bson.A{}

	for key, value := range fields {
		fieldsBson = append(fieldsBson, bson.D{{Key: key, Value: value}})
	}

	filter := bson.D{
		{Key: "$or", Value: fieldsBson},
		{Key: "deleted_at", Value: bson.D{{Key: "$eq", Value: time.Time{}}}},
	}

	data := new(T)

	err := database.collection.FindOne(context.Background(), filter).Decode(data)
	if err != nil {
		return nil, fmt.Errorf("error getting data from database: %w", err)
	}

	return data, nil
}

func (database *mongo[T]) update(dataID uuid.UUID, fields map[string]any) error {
	fieldsBson := bson.D{}

	for key, value := range fields {
		fieldsBson = append(fieldsBson, bson.E{Key: key, Value: value})
	}

	update := bson.D{{Key: "$set", Value: fieldsBson}}

	_, err := database.collection.UpdateByID(context.Background(), dataID, update)
	if err != nil {
		return fmt.Errorf("error getting data from database: %w", err)
	}

	return nil
}

func (database *mongo[T]) getAll() ([]T, error) {
	data := []T{}

	cursor, err := database.collection.Find(context.Background(), bson.D{})
	if err != nil {
		return nil, fmt.Errorf("error getting all data from database: %w", err)
	}

	err = cursor.All(context.Background(), &data)
	if err != nil {
		return nil, fmt.Errorf("error parsing data: %w", err)
	}

	return data, nil
}

func createMongoDatabase[T any](client *mongodb.Client, database, collection string) *mongo[T] {
	return &mongo[T]{client.Database(database).Collection(collection)}
}

type User struct {
	users    *mongo[model.User]
	sessions *mongo[model.UserSession]
	roles    *mongo[model.Role]
}

func (database *User) Create(user model.User) error {
	return database.users.create(user)
}

func (database *User) ExistByID(userID uuid.UUID) (bool, error) {
	return database.users.existByID(userID)
}

func (database *User) ExistByNameOrEmail(name, email string) (bool, error) {
	filter := map[string]any{
		"name":  name,
		"email": email,
	}

	return database.users.existByFieldsOr(filter)
}

func (database *User) GetByID(userID uuid.UUID) (*model.User, error) {
	return database.users.getByID(userID)
}

func (database *User) GetByNameOrEmail(name, email string) (*model.User, error) {
	filter := map[string]any{
		"name":  name,
		"email": email,
	}

	return database.users.getByFieldsOr(filter)
}

func (database *User) GetAll() ([]model.User, error) {
	return database.users.getAll()
}

func (database *User) Update(user model.User) error {
	update := map[string]any{
		"password":   user.Password,
		"deleted_at": user.DeletedAt,
		"deleted_by": user.DeletedBy,
		"is_admin":   user.IsAdmin,
		"protected":  user.IsProtected,
		"roles":      user.Roles,
	}

	return database.users.update(user.ID, update)
}

func (database *User) SaveSession(session model.UserSession) error {
	return database.sessions.create(session)
}

func (database *User) ExistSession(sessionID uuid.UUID) (bool, error) {
	return database.sessions.existByID(sessionID)
}

func (database *User) GetSession(sessionID uuid.UUID) (*model.UserSession, error) {
	return database.sessions.getByID(sessionID)
}

func (database *User) UpdateSession(session model.UserSession) error {
	update := map[string]any{"deleted_at": session.DeletedAt}

	return database.sessions.update(session.ID, update)
}

func (database *User) CreateRole(role model.Role) error {
	return database.roles.create(role)
}

func (database *User) GetRole(name string) (*model.Role, error) {
	filter := map[string]any{"name": name}

	return database.roles.getByFieldsOr(filter)
}

func (database *User) GetAllRoles() ([]model.Role, error) {
	return database.roles.getAll()
}

func newUserDatabase(client *mongodb.Client) *User {
	return &User{
		createMongoDatabase[model.User](client, "users", "users"),
		createMongoDatabase[model.UserSession](client, "users", "sessions"),
		createMongoDatabase[model.Role](client, "users", "roles"),
	}
}

type Queue struct {
	queues *mongo[model.Queue]
	emails *mongo[model.Email]
}

func (database *Queue) Create(queue model.Queue) error {
	return database.queues.create(queue)
}

func (database *Queue) Get(name string) (*model.Queue, error) {
	filter := map[string]any{"name": name}

	return database.queues.getByFieldsOr(filter)
}

func (database *Queue) GetAll() ([]model.Queue, error) {
	return database.queues.getAll()
}

func (database *Queue) Exist(name string) (bool, error) {
	filter := map[string]any{
		"name": name,
		"dlx":  name,
	}

	return database.queues.existByFieldsOr(filter)
}

func (database *Queue) Update(queue model.Queue) error {
	update := map[string]any{
		"deleted_at": queue.DeletedAt,
		"deleted_by": queue.DeletedBy,
	}

	return database.queues.update(queue.ID, update)
}

func (database *Queue) SaveEmail(email model.Email) error {
	return database.emails.create(email)
}

func newQueueDatabase(client *mongodb.Client) *Queue {
	return &Queue{
		createMongoDatabase[model.Queue](client, "email", "queues"),
		createMongoDatabase[model.Email](client, "email", "sent"),
	}
}

type Template struct {
	templates *mongo[model.Template]
}

func (database *Template) Create(template model.Template) error {
	return database.templates.create(template)
}

func (database *Template) Update(template model.Template) error {
	update := map[string]any{
		"template":  template.Template,
		"fields":    template.Fields,
		"createdAt": template.CreatedAt,
		"createdBy": template.CreatedBy,
		"deletedAt": template.DeletedAt,
		"deletedBy": template.DeletedBy,
	}

	return database.templates.update(template.ID, update)
}

func (database *Template) Exist(name string) (bool, error) {
	filter := map[string]any{"name": name}

	return database.templates.existByFieldsOr(filter)
}

func (database *Template) Get(name string) (*model.Template, error) {
	filter := map[string]any{"name": name}

	return database.templates.getByFieldsOr(filter)
}

func (database *Template) GetAll() ([]model.Template, error) {
	return database.templates.getAll()
}

func newTemplateDatabase(client *mongodb.Client) *Template {
	return &Template{
		createMongoDatabase[model.Template](client, "template", "templates"),
	}
}

func NewMongoClient(uri string) (*mongodb.Client, error) {
	connection, err := mongodb.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("error connecting with the database: %w", err)
	}

	err = connection.Ping(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("error ping server: %w", err)
	}

	return connection, nil
}

type Databases struct {
	*User
	*Queue
	*Template
}

func NewDatabases(client *mongodb.Client) *Databases {
	return &Databases{
		User:     newUserDatabase(client),
		Queue:    newQueueDatabase(client),
		Template: newTemplateDatabase(client),
	}
}
