package repository

import (
	"live-chatter/pkg/db"
	"live-chatter/pkg/model"
	"time"

	"gorm.io/gorm"
)

type MessageRepository interface {
	CreateMessage(message *model.Message) error
	GetMessagesByRoomID(roomID string, limit, offset int, before *time.Time) ([]model.Message, error)
	SearchMessages(query, roomID string, limit int) ([]model.Message, error)
	GetMessageByID(messageID uint) (*model.Message, error)
	UpdateMessage(message *model.Message) error
	DeleteMessage(messageID uint) error
	GetMessageCountByRoom(roomID string) (int64, error)
}

type messageRepository struct {
	db *gorm.DB
}

func NewMessageRepository() MessageRepository {
	return &messageRepository{db: db.GetDB()}
}

func (r *messageRepository) CreateMessage(message *model.Message) error {
	return r.db.Create(message).Error
}

func (r *messageRepository) GetMessagesByRoomID(roomID string, limit, offset int, before *time.Time) ([]model.Message, error) {
	var messages []model.Message

	query := r.db.Preload("User").Where("room_id = ? AND deleted_at IS NULL", roomID)

	if before != nil {
		query = query.Where("created_at < ?", before)
	}

	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error

	return messages, err
}

func (r *messageRepository) SearchMessages(query, roomID string, limit int) ([]model.Message, error) {
	var messages []model.Message

	dbQuery := r.db.Preload("User").Where("content ILIKE ? AND deleted_at IS NULL", "%"+query+"%")

	if roomID != "" {
		dbQuery = dbQuery.Where("room_id = ?", roomID)
	}

	err := dbQuery.Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error

	return messages, err
}

func (r *messageRepository) GetMessageByID(messageID uint) (*model.Message, error) {
	var message model.Message
	err := r.db.Preload("User").Preload("Parent").Preload("Replies").
		First(&message, messageID).Error
	return &message, err
}

func (r *messageRepository) UpdateMessage(message *model.Message) error {
	// Set edited flag and timestamp
	now := time.Now()
	message.Edited = true
	message.EditedAt = &now

	return r.db.Save(message).Error
}

func (r *messageRepository) DeleteMessage(messageID uint) error {
	// Soft delete the message
	return r.db.Delete(&model.Message{}, messageID).Error
}

func (r *messageRepository) GetMessageCountByRoom(roomID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.Message{}).
		Where("room_id = ? AND deleted_at IS NULL", roomID).
		Count(&count).Error
	return count, err
}
