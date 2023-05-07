package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/thiago-felipe-99/mail/publisher/data"
	"github.com/thiago-felipe-99/mail/publisher/model"
	"go.mongodb.org/mongo-driver/mongo"
)

type User struct {
	database        *data.User
	validator       *validator.Validate
	argon2id        argon2id.Params
	durationSession time.Duration
}

func (core *User) existByID(userID uuid.UUID) (bool, error) {
	exist, err := core.database.ExistByID(userID)
	if err != nil {
		return false, fmt.Errorf("error checking if user exist in database: %w", err)
	}

	return exist, nil
}

func (core *User) ExistByNameOrEmail(name, email string) (bool, error) {
	exist, err := core.database.ExistByNameOrEmail(name, email)
	if err != nil {
		return false, fmt.Errorf("error checking if user exist in database: %w", err)
	}

	return exist, nil
}

func (core *User) Create(partial model.UserPartial, adminID uuid.UUID) error {
	err := validate(core.validator, partial)
	if err != nil {
		return err
	}

	exist, err := core.ExistByNameOrEmail(partial.Name, partial.Email)
	if err != nil {
		return fmt.Errorf("error checking if user exist in database: %w", err)
	}

	if exist {
		return ErrUserAlreadyExist
	}

	hash, err := argon2id.CreateHash(partial.Password, &core.argon2id)
	if err != nil {
		return fmt.Errorf("error creating password hash: %w", err)
	}

	user := model.User{
		ID:        uuid.New(),
		Name:      partial.Name,
		Email:     partial.Email,
		Password:  hash,
		Roles:     []model.UserRole{},
		CreatedAt: time.Now(),
		CreatedBy: adminID,
		IsAdmin:   false,
		DeletedAt: time.Time{},
		DeletedBy: uuid.UUID{},
	}

	err = core.database.Create(user)
	if err != nil {
		return fmt.Errorf("error creating user in database: %w", err)
	}

	return nil
}

func (core *User) GetByID(userID uuid.UUID) (*model.User, error) {
	exist, err := core.existByID(userID)
	if err != nil {
		return nil, fmt.Errorf("error checking if user exist in database: %w", err)
	}

	if !exist {
		return nil, ErrUserDoesNotExist
	}

	user, err := core.database.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("error getting user from database: %w", err)
	}

	return user, nil
}

func (core *User) GetAll() ([]model.User, error) {
	users, err := core.database.GetAll()
	if err != nil {
		return nil, fmt.Errorf("error getting alls users from database: %w", err)
	}

	return users, nil
}

func (core *User) GetByNameOrEmail(name, email string) (*model.User, error) {
	exist, err := core.ExistByNameOrEmail(name, email)
	if err != nil {
		return nil, fmt.Errorf("error checking if user exist in database: %w", err)
	}

	if !exist {
		return nil, ErrUserDoesNotExist
	}

	user, err := core.database.GetByNameOrEmail(name, email)
	if err != nil {
		return nil, fmt.Errorf("error getting user from database: %w", err)
	}

	return user, nil
}

func (core *User) Update(userID uuid.UUID, partial model.UserPartial) error {
	err := validate(core.validator, partial)
	if err != nil {
		return err
	}

	user, err := core.GetByID(userID)
	if err != nil {
		return fmt.Errorf("error checking if user exist in database: %w", err)
	}

	hash, err := argon2id.CreateHash(partial.Password, &core.argon2id)
	if err != nil {
		return fmt.Errorf("error creating password hash: %w", err)
	}

	user.Password = hash

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error updating user in database: %w", err)
	}

	return nil
}

func (core *User) Delete(userID uuid.UUID, deleteByID uuid.UUID) error {
	user, err := core.GetByID(userID)
	if err != nil {
		return err
	}

	if user.IsProtected {
		return ErrUserIsProtected
	}

	user.DeletedAt = time.Now()
	user.DeletedBy = deleteByID

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error deleting user from database: %w", err)
	}

	return nil
}

func (core *User) IsAdmin(userID uuid.UUID) (bool, error) {
	user, err := core.GetByID(userID)
	if err != nil {
		return false, err
	}

	return user.IsAdmin, nil
}

func (core *User) NewAdmin(userID uuid.UUID) error {
	user, err := core.GetByID(userID)
	if err != nil {
		return err
	}

	user.IsAdmin = true

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error updating user in database: %w", err)
	}

	return nil
}

func (core *User) RemoveAdmin(userID uuid.UUID) error {
	user, err := core.GetByID(userID)
	if err != nil {
		return err
	}

	if user.IsProtected {
		return ErrUserIsProtected
	}

	if !user.IsAdmin {
		return ErrUserIsNotAdmin
	}

	user.IsAdmin = false

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error updating user in database: %w", err)
	}

	return nil
}

func (core *User) Protected(userID uuid.UUID) error {
	user, err := core.GetByID(userID)
	if err != nil {
		return err
	}

	user.IsAdmin = true
	user.IsProtected = true

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error updating user in database: %w", err)
	}

	return nil
}

func (core *User) GetRoles(userID uuid.UUID) ([]model.UserRole, error) {
	user, err := core.GetByID(userID)
	if err != nil {
		return nil, err
	}

	return user.Roles, nil
}

func (core *User) existRole(role string) (bool, error) {
	_, err := core.database.GetRole(role)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}

		return false, fmt.Errorf("error getting role from database: %w", err)
	}

	return true, nil
}

func existsInSlice[T comparable](slice []T, find T) (int, bool) {
	for index, element := range slice {
		if element == find {
			return index, true
		}
	}

	return 0, false
}

func (core *User) existRoles(roles []string) (bool, error) {
	if len(roles) == 0 {
		return true, nil
	}

	rolesRaw, err := core.database.GetAllRoles()
	if err != nil {
		return false, fmt.Errorf("error getting role from database: %w", err)
	}

	rolesNames := make([]string, 0, len(rolesRaw))
	for _, role := range rolesRaw {
		rolesNames = append(rolesNames, role.Name)
	}

	for _, role := range roles {
		if _, exist := existsInSlice(rolesNames, role); !exist {
			return false, nil
		}
	}

	return true, nil
}

func (core *User) CreateRole(partial model.RolePartial, userID uuid.UUID) error {
	err := validate(core.validator, partial)
	if err != nil {
		return err
	}

	exist, err := core.existRole(partial.Name)
	if err != nil {
		return fmt.Errorf("error checking if role exist: %w", err)
	}

	if exist {
		return ErrRoleAlreadyExist
	}

	role := model.Role{
		ID:        uuid.New(),
		Name:      partial.Name,
		CreatedAt: time.Now(),
		CreatedBy: userID,
		DeletedAt: time.Time{},
		DeletedBy: uuid.UUID{},
	}

	err = core.database.CreateRole(role)
	if err != nil {
		return fmt.Errorf("error creating role in database: %w", err)
	}

	return nil
}

func (core *User) HasRoles(userID uuid.UUID, roles []model.UserRole) (bool, error) {
	if len(roles) == 0 {
		return true, nil
	}

	user, err := core.GetByID(userID)
	if err != nil {
		return false, err
	}

	if user.IsAdmin {
		return true, nil
	}

	userRoles := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		userRoles = append(userRoles, role.Name)
	}

	for _, role := range roles {
		index, exist := existsInSlice(userRoles, role.Name)
		if !exist {
			return false, nil
		}

		if (role.IsAdmin && !user.Roles[index].IsAdmin) ||
			(role.IsProtected && !user.Roles[index].IsProtected) {
			return false, nil
		}
	}

	return true, nil
}

func (core *User) HasRolesAdmin(userID uuid.UUID, roles []model.RolePartial) (bool, error) {
	if len(roles) == 0 {
		return true, nil
	}

	user, err := core.GetByID(userID)
	if err != nil {
		return false, err
	}

	if user.IsAdmin {
		return true, nil
	}

	userRoles := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		userRoles = append(userRoles, role.Name)
	}

	for _, role := range roles {
		index, exist := existsInSlice(userRoles, role.Name)
		if !exist {
			return false, nil
		}

		if !user.Roles[index].IsAdmin {
			return false, nil
		}
	}

	return true, nil
}

func (core *User) AddRoles(roles []model.UserRole, userID uuid.UUID) error {
	if len(roles) == 0 {
		return nil
	}

	user, err := core.GetByID(userID)
	if err != nil {
		return err
	}

	rolesName := make([]string, 0, len(roles))
	for _, role := range roles {
		rolesName = append(rolesName, role.Name)
	}

	exist, err := core.existRoles(rolesName)
	if err != nil {
		return fmt.Errorf("eror checking if roles exist: %w", err)
	}

	if !exist {
		return ErrRoleDoesNotExist
	}

	userRolesName := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		userRolesName = append(userRolesName, role.Name)
	}

	for _, role := range roles {
		index, exist := existsInSlice(userRolesName, role.Name)
		if !exist {
			newRole := model.UserRole{
				Name:        role.Name,
				IsAdmin:     role.IsAdmin || role.IsProtected,
				IsProtected: role.IsProtected,
			}

			user.Roles = append(user.Roles, newRole)
		} else if !user.Roles[index].IsProtected {
			user.Roles[index].IsAdmin = role.IsAdmin || role.IsProtected
			user.Roles[index].IsProtected = role.IsProtected
		}
	}

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error updating user: %w", err)
	}

	return nil
}

func (core *User) DeleteRoles(roles []model.RolePartial, userID uuid.UUID, protected bool) error {
	if len(roles) == 0 {
		return nil
	}

	user, err := core.GetByID(userID)
	if err != nil {
		return err
	}

	rolesName := make([]string, 0, len(roles))
	for _, role := range roles {
		rolesName = append(rolesName, role.Name)
	}

	exist, err := core.existRoles(rolesName)
	if err != nil {
		return fmt.Errorf("eror checking if roles exist: %w", err)
	}

	if !exist {
		return ErrRoleDoesNotExist
	}

	userRolesName := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		userRolesName = append(userRolesName, role.Name)
	}

	for _, role := range roles {
		index, exist := existsInSlice(userRolesName, role.Name)
		if exist && (!user.Roles[index].IsProtected || protected) {
			lastIndex := len(user.Roles) - 1

			user.Roles[index] = user.Roles[lastIndex]
			user.Roles = user.Roles[:lastIndex]

			userRolesName[index] = userRolesName[lastIndex]
			userRolesName = userRolesName[:lastIndex]
		}
	}

	err = core.database.Update(*user)
	if err != nil {
		return fmt.Errorf("error updating user: %w", err)
	}

	return nil
}

func (core *User) NewSession(partial model.UserSessionPartial) (*model.UserSession, error) {
	err := validate(core.validator, partial)
	if err != nil {
		return nil, err
	}

	exist, err := core.ExistByNameOrEmail(partial.Name, partial.Email)
	if err != nil {
		return nil, fmt.Errorf("error checking if user exist in database: %w", err)
	}

	if !exist {
		return nil, ErrUserDoesNotExist
	}

	user, err := core.database.GetByNameOrEmail(partial.Name, partial.Email)
	if err != nil {
		return nil, fmt.Errorf("error getting user in database: %w", err)
	}

	equals, err := argon2id.ComparePasswordAndHash(partial.Password, user.Password)
	if err != nil {
		return nil, fmt.Errorf("error comparing password with hash: %w", err)
	}

	if !equals {
		return nil, ErrUserWrongPassword
	}

	session := model.UserSession{
		ID:        uuid.New(),
		UserID:    user.ID,
		CreateaAt: time.Now(),
		Expires:   time.Now().Add(core.durationSession),
		DeletedAt: time.Now().Add(core.durationSession),
	}

	err = core.database.SaveSession(session)
	if err != nil {
		return nil, fmt.Errorf("error saving session in database: %w", err)
	}

	return &session, nil
}

func (core *User) RefreshSession(sessionID string) (*model.UserSession, error) {
	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, ErrInvalidID
	}

	exist, err := core.database.ExistSession(sessionUUID)
	if err != nil {
		return nil, fmt.Errorf("error checking if session exist in database: %w", err)
	}

	if !exist {
		return nil, ErrUserSessionDoesNotExist
	}

	currentSession, err := core.database.GetSession(sessionUUID)
	if err != nil {
		return nil, fmt.Errorf("error getting session from database: %w", err)
	}

	if currentSession.DeletedAt.Before(time.Now()) {
		return nil, ErrUserSessionDoesNotExist
	}

	currentSession.DeletedAt = time.Now()

	exist, err = core.existByID(currentSession.UserID)
	if err != nil {
		return nil, fmt.Errorf("error checking if user exist in database: %w", err)
	}

	if !exist {
		return nil, ErrUserSessionDoesNotExist
	}

	err = core.database.UpdateSession(*currentSession)
	if err != nil {
		return nil, fmt.Errorf("error updating session in database: %w", err)
	}

	newSession := model.UserSession{
		ID:        uuid.New(),
		UserID:    currentSession.UserID,
		CreateaAt: time.Now(),
		Expires:   time.Now().Add(core.durationSession),
		DeletedAt: time.Now().Add(core.durationSession),
	}

	err = core.database.SaveSession(newSession)
	if err != nil {
		return nil, fmt.Errorf("error saving session in database: %w", err)
	}

	return &newSession, nil
}

func NewUser(
	database *data.User,
	validate *validator.Validate,
	durationSession time.Duration,
) *User {
	return &User{
		database:  database,
		validator: validate,
		argon2id: argon2id.Params{
			Memory:      argon2idParamMemory,
			Iterations:  argon2idParamIterations,
			Parallelism: argon2idParamParallelism,
			SaltLength:  argon2idParamSaltLength,
			KeyLength:   argon2idParamKeyLength,
		},
		durationSession: durationSession,
	}
}
