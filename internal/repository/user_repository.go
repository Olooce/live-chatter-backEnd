package repository

import (
	"live-chatter/pkg/db"
	"live-chatter/pkg/model"
)

type UserRepository interface {
	CreateUser(user *model.User) error
	GetUserByEmail(email string) (*model.User, error)
	GetAllUsers() ([]model.User, error)
	GetOnlineUsers() ([]model.User, error)
	UpdateUserStatus(userID uint, status string) error
	GetUserByUsername(username string) (*model.User, error)
	GetUserByID(id uint) (*model.User, error)
}

type userRepository struct{}

func (r *userRepository) GetUserByID(id uint) (*model.User, error) {
	var user model.User
	err := db.GetDB().Where("id = ?", id).First(&user).Error
	return &user, err
}

func NewUserRepository() UserRepository {
	return &userRepository{}
}

func (r *userRepository) CreateUser(user *model.User) error {
	return db.GetDB().Create(user).Error
}

func (r *userRepository) GetUserByEmail(email string) (*model.User, error) {
	var user model.User
	err := db.GetDB().Where("email = ?", email).First(&user).Error
	return &user, err
}

func (r *userRepository) GetAllUsers() ([]model.User, error) {
	var users []model.User
	err := db.GetDB().Find(&users).Error
	return users, err
}

func (r *userRepository) GetOnlineUsers() ([]model.User, error) {
	var users []model.User
	err := db.GetDB().Where("status = ?", "online").Find(&users).Error
	return users, err
}

func (r *userRepository) UpdateUserStatus(userID uint, status string) error {
	return db.GetDB().Model(&model.User{}).Where("id = ?", userID).Update("status", status).Error
}

func (r *userRepository) GetUserByUsername(username string) (*model.User, error) {
	var user model.User
	err := db.GetDB().Where("username = ?", username).First(&user).Error
	return &user, err
}
