package auths

import (
	"context"

	"model"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/gorms"
	"gorm.io/gorm"
)

type Repository interface {
	transaction(f func(tx *gorm.DB) error) error
	findByUserName(ctx context.Context, username string) (*model.User, error)
	findByEmail(ctx context.Context, email string) (*model.User, error)
	findById(ctx context.Context, id uuid.UUID) (*model.User, error)
	findByUserNameOrEmail(ctx context.Context, username string) (*model.User, error)
	saveUser(ctx context.Context, tx *gorm.DB, u *model.User) error
	updateUser(ctx context.Context, tx *gorm.DB, u *model.User) error
}

type Model struct {
	db *gorm.DB
}

func NewModel() *Model {
	return &Model{
		db: database.GetPostgresDB().GormDB,
	}
}

func (m *Model) findByUserName(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := m.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &user, err
}

func (m *Model) findByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := m.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &user, err
}

func (m *Model) findById(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := m.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &user, err
}

func (m *Model) findByUserNameOrEmail(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := m.db.WithContext(ctx).Where("username = ? or email = ?", username, username).First(&user).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &user, err
}

func (m *Model) saveUser(ctx context.Context, tx *gorm.DB, u *model.User) error {
	if tx == nil {
		tx = m.db
	}
	return tx.WithContext(ctx).Create(u).Error
}

func (m *Model) updateUser(ctx context.Context, tx *gorm.DB, u *model.User) error {
	return tx.WithContext(ctx).Save(u).Error
}

func (m *Model) transaction(f func(tx *gorm.DB) error) error {
	return m.db.Transaction(f)
}
