package repository

import (
	"errors"
	"live-chatter/pkg/db"
	"live-chatter/pkg/model"
	"time"

	"gorm.io/gorm"
)

type RoomRepository interface {
	CreateRoom(room *model.Room) error
	GetAllRooms() ([]model.Room, error)
	GetRoomByID(roomID string) (*model.Room, error)
	GetRoomByName(name string) (*model.Room, error)
	GetUserRooms(userID uint) ([]model.Room, error)
	AddUserToRoom(roomID string, userID uint, role string) error
	RemoveUserFromRoom(roomID string, userID uint) error
	IsUserInRoom(roomID string, userID uint) (bool, error)
	UpdateRoom(room *model.Room) error
	DeleteRoom(roomID string) error
}

type roomRepository struct {
	db *gorm.DB
}

func NewRoomRepository() RoomRepository {
	return &roomRepository{db: db.GetDB()}
}

func (r *roomRepository) CreateRoom(room *model.Room) error {
	return r.db.Create(room).Error
}

func (r *roomRepository) GetAllRooms() ([]model.Room, error) {
	var rooms []model.Room
	err := r.db.Preload("Creator").Find(&rooms).Error
	return rooms, err
}

func (r *roomRepository) GetRoomByID(roomID string) (*model.Room, error) {
	var room model.Room
	err := r.db.Preload("Creator").First(&room, "id = ?", roomID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &room, err
}

func (r *roomRepository) GetRoomByName(name string) (*model.Room, error) {
	var room model.Room
	err := r.db.First(&room, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &room, err
}

func (r *roomRepository) GetUserRooms(userID uint) ([]model.Room, error) {
	var rooms []model.Room
	err := r.db.Table("rooms").
		Joins("JOIN user_rooms ON user_rooms.room_id = rooms.id").
		Where("user_rooms.user_id = ? AND user_rooms.left_at IS NULL", userID).
		Preload("Creator").
		Find(&rooms).Error
	return rooms, err
}

func (r *roomRepository) AddUserToRoom(roomID string, userID uint, role string) error {
	var existingUserRoom model.UserRoom
	err := r.db.Where("room_id = ? AND user_id = ? AND left_at IS NULL", roomID, userID).First(&existingUserRoom).Error

	if err == nil {
		return nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	var previousUserRoom model.UserRoom
	err = r.db.Where("room_id = ? AND user_id = ?", roomID, userID).First(&previousUserRoom).Error

	if err == nil {
		return r.db.Model(&previousUserRoom).Updates(map[string]interface{}{
			"role":      role,
			"joined_at": time.Now(),
			"left_at":   nil,
		}).Error
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	userRoom := model.UserRoom{
		UserID:   userID,
		RoomID:   roomID,
		Role:     role,
		JoinedAt: time.Now(),
	}

	return r.db.Create(&userRoom).Error
}

func (r *roomRepository) RemoveUserFromRoom(roomID string, userID uint) error {
	return r.db.Model(&model.UserRoom{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Update("left_at", time.Now()).Error
}

func (r *roomRepository) IsUserInRoom(roomID string, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&model.UserRoom{}).
		Where("room_id = ? AND user_id = ? AND left_at IS NULL", roomID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *roomRepository) UpdateRoom(room *model.Room) error {
	return r.db.Save(room).Error
}

func (r *roomRepository) DeleteRoom(roomID string) error {
	// First mark all users as left
	if err := r.db.Model(&model.UserRoom{}).
		Where("room_id = ?", roomID).
		Update("left_at", time.Now()).Error; err != nil {
		return err
	}

	// Then soft delete the room
	return r.db.Delete(&model.Room{}, "id = ?", roomID).Error
}
