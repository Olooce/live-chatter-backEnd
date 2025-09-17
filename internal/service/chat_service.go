package service

import (
	"errors"
	"fmt"
	"time"

	"live-chatter/internal/repository"
	"live-chatter/pkg/model"

	"github.com/google/uuid"
)

type ChatService interface {
	// Room management
	CreateRoom(room *model.Room) (*model.Room, error)
	GetAllRooms() ([]model.Room, error)
	GetRoomByID(roomID string) (*model.Room, error)
	GetUserRooms(userID uint) ([]model.Room, error)
	JoinRoom(roomID string, userID uint) error
	LeaveRoom(roomID string, userID uint) error

	// Message management
	SaveMessage(message *model.Message) (*model.Message, error)
	GetRoomMessages(roomID string, limit, offset int, before *time.Time) ([]model.Message, error)
	SearchMessages(query, roomID string, limit int) ([]model.Message, error)

	// User management
	GetOnlineUsers() ([]model.User, error)
	UpdateUserStatus(userID uint, status string) error
	GetUserByUsername(username string) (*model.User, error)
}

type chatService struct {
	messageRepo repository.MessageRepository
	roomRepo    repository.RoomRepository
	userRepo    repository.UserRepository
}

func NewChatService(messageRepo repository.MessageRepository, roomRepo repository.RoomRepository, userRepo repository.UserRepository) ChatService {
	return &chatService{
		messageRepo: messageRepo,
		roomRepo:    roomRepo,
		userRepo:    userRepo,
	}
}

// CreateRoom creates a new chat room
func (s *chatService) CreateRoom(room *model.Room) (*model.Room, error) {
	// Generate UUID for room ID
	room.ID = uuid.New().String()

	// Validate room name
	if room.Name == "" {
		return nil, errors.New("room name cannot be empty")
	}

	// Check if room name already exists
	existingRoom, _ := s.roomRepo.GetRoomByName(room.Name)
	if existingRoom != nil {
		return nil, errors.New("room name already exists")
	}

	// Set default type if not provided
	if room.Type == "" {
		room.Type = "public"
	}

	// Create the room
	err := s.roomRepo.CreateRoom(room)
	if err != nil {
		return nil, fmt.Errorf("failed to create room: %v", err)
	}

	// Automatically join the creator to the room
	err = s.roomRepo.AddUserToRoom(room.ID, room.CreatedBy, "admin")
	if err != nil {
		return nil, fmt.Errorf("failed to add creator to room: %v", err)
	}

	return room, nil
}

// GetAllRooms returns all available rooms
func (s *chatService) GetAllRooms() ([]model.Room, error) {
	return s.roomRepo.GetAllRooms()
}

// GetRoomByID returns a room by its ID
func (s *chatService) GetRoomByID(roomID string) (*model.Room, error) {
	return s.roomRepo.GetRoomByID(roomID)
}

// GetUserRooms returns rooms that a user has joined
func (s *chatService) GetUserRooms(userID uint) ([]model.Room, error) {
	return s.roomRepo.GetUserRooms(userID)
}

// JoinRoom adds a user to a room
func (s *chatService) JoinRoom(roomID string, userID uint) error {
	// Check if room exists
	room, err := s.roomRepo.GetRoomByID(roomID)
	if err != nil {
		return errors.New("room not found")
	}

	if room == nil {
		return errors.New("room not found")
	}

	// Check if user is already in the room
	isInRoom, err := s.roomRepo.IsUserInRoom(roomID, userID)
	if err != nil {
		return fmt.Errorf("failed to check room membership: %v", err)
	}

	if isInRoom {
		return errors.New("user already in room")
	}

	// Add user to room
	err = s.roomRepo.AddUserToRoom(roomID, userID, "member")
	if err != nil {
		return fmt.Errorf("failed to join room: %v", err)
	}

	return nil
}

// LeaveRoom removes a user from a room
func (s *chatService) LeaveRoom(roomID string, userID uint) error {
	// Check if room exists
	room, err := s.roomRepo.GetRoomByID(roomID)
	if err != nil {
		return errors.New("room not found")
	}

	if room == nil {
		return errors.New("room not found")
	}

	// Check if user is in the room
	isInRoom, err := s.roomRepo.IsUserInRoom(roomID, userID)
	if err != nil {
		return fmt.Errorf("failed to check room membership: %v", err)
	}

	if !isInRoom {
		return errors.New("user is not in this room")
	}

	// Remove user from room
	err = s.roomRepo.RemoveUserFromRoom(roomID, userID)
	if err != nil {
		return fmt.Errorf("failed to leave room: %v", err)
	}

	return nil
}

func (s *chatService) GetOnlineUsers() ([]model.User, error) {
	return s.userRepo.GetOnlineUsers()
}

func (s *chatService) SaveMessage(message *model.Message) (*model.Message, error) {
	// Validate message content
	if message.Content == "" {
		return nil, errors.New("message content cannot be empty")
	}

	// Validate room exists
	room, err := s.roomRepo.GetRoomByID(message.RoomID)
	if err != nil || room == nil {
		return nil, errors.New("room not found")
	}

	// Check if user is in the room
	isInRoom, err := s.roomRepo.IsUserInRoom(message.RoomID, message.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to check room membership: %v", err)
	}
	if !isInRoom {
		return nil, errors.New("user is not in this room")
	}

	// Set message timestamp
	message.CreatedAt = time.Now()

	// Save message
	err = s.messageRepo.CreateMessage(message)
	if err != nil {
		return nil, fmt.Errorf("failed to save message: %v", err)
	}

	return message, nil
}

func (s *chatService) GetRoomMessages(roomID string, limit, offset int, before *time.Time) ([]model.Message, error) {
	// Validate room exists
	room, err := s.roomRepo.GetRoomByID(roomID)
	if err != nil || room == nil {
		return nil, errors.New("room not found")
	}

	// Get messages
	messages, err := s.messageRepo.GetMessagesByRoomID(roomID, limit, offset, before)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %v", err)
	}

	return messages, nil
}

func (s *chatService) SearchMessages(query, roomID string, limit int) ([]model.Message, error) {
	// Validate query
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}

	// If roomID is provided, validate room exists
	if roomID != "" {
		room, err := s.roomRepo.GetRoomByID(roomID)
		if err != nil || room == nil {
			return nil, errors.New("room not found")
		}
	}

	// Search messages
	messages, err := s.messageRepo.SearchMessages(query, roomID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %v", err)
	}

	return messages, nil
}

func (s *chatService) UpdateUserStatus(userID uint, status string) error {
	// Validate status
	validStatuses := map[string]bool{
		"online":  true,
		"offline": true,
		"away":    true,
		"busy":    true,
	}
	if !validStatuses[status] {
		return errors.New("invalid status")
	}

	// Update user status
	err := s.userRepo.UpdateUserStatus(userID, status)
	if err != nil {
		return fmt.Errorf("failed to update user status: %v", err)
	}

	return nil
}

func (s *chatService) GetUserByUsername(username string) (*model.User, error) {
	// Validate username
	if username == "" {
		return nil, errors.New("username cannot be empty")
	}

	// Get user
	user, err := s.userRepo.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	if user == nil {
		return nil, errors.New("user not found")
	}

	return user, nil
}
