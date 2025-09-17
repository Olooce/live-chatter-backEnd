package model

import (
	"time"

	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Username  string         `json:"username" gorm:"uniqueIndex;not null"`
	Email     string         `json:"email" gorm:"uniqueIndex;not null"`
	Password  string         `json:"password,omitempty" gorm:"not null"` // Exclude from JSON responses
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	Status    string         `json:"status" gorm:"default:'offline'"` // online, offline, away, busy
	LastSeen  *time.Time     `json:"last_seen"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	Messages     []Message `json:"-" gorm:"foreignKey:UserID"`
	Rooms        []Room    `json:"-" gorm:"many2many:user_rooms;"`
	SentMessages []Message `json:"-" gorm:"foreignKey:UserID"`
}

// Room represents a chat room
type Room struct {
	ID          string         `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"not null"`
	Description string         `json:"description"`
	Type        string         `json:"type" gorm:"default:'public'"` // public, private
	CreatedBy   uint           `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	Creator  User      `json:"creator" gorm:"foreignKey:CreatedBy"`
	Users    []User    `json:"users" gorm:"many2many:user_rooms;"`
	Messages []Message `json:"messages" gorm:"foreignKey:RoomID"`
}

// Message represents a chat message
type Message struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Content   string         `json:"content" gorm:"not null"`
	Type      string         `json:"type" gorm:"default:'text'"` // text, image, file, system
	UserID    uint           `json:"user_id"`
	Username  string         `json:"username"`
	RoomID    string         `json:"room_id"`
	ParentID  *uint          `json:"parent_id"` // For threaded messages
	Edited    bool           `json:"edited" gorm:"default:false"`
	EditedAt  *time.Time     `json:"edited_at"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	User    User      `json:"user" gorm:"foreignKey:UserID"`
	Room    Room      `json:"room" gorm:"foreignKey:RoomID"`
	Parent  *Message  `json:"parent" gorm:"foreignKey:ParentID"`
	Replies []Message `json:"replies" gorm:"foreignKey:ParentID"`
}

// UserRoom represents the many-to-many relationship between users and rooms
type UserRoom struct {
	UserID   uint      `gorm:"primaryKey"`
	RoomID   string    `gorm:"primaryKey"`
	Role     string    `gorm:"default:'member'"` // admin, moderator, member
	JoinedAt time.Time `gorm:"autoCreateTime"`
	LeftAt   *time.Time

	User User `gorm:"foreignKey:UserID"`
	Room Room `gorm:"foreignKey:RoomID"`
}

// PrivateMessage represents direct messages between users
type PrivateMessage struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Content     string         `json:"content" gorm:"not null"`
	Type        string         `json:"type" gorm:"default:'text'"`
	SenderID    uint           `json:"sender_id"`
	RecipientID uint           `json:"recipient_id"`
	Read        bool           `json:"read" gorm:"default:false"`
	ReadAt      *time.Time     `json:"read_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	Sender    User `json:"sender" gorm:"foreignKey:SenderID"`
	Recipient User `json:"recipient" gorm:"foreignKey:RecipientID"`
}

// UserSession represents active user sessions
type UserSession struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id"`
	Token     string    `json:"-" gorm:"uniqueIndex"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`

	User User `json:"user" gorm:"foreignKey:UserID"`
}

// ActivityLog represents user activity logging
type ActivityLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id"`
	Action    string    `json:"action"` // login, logout, join_room, leave_room, send_message
	Details   string    `json:"details"`
	IPAddress string    `json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`

	User User `json:"user" gorm:"foreignKey:UserID"`
}

// TableName methods for custom table names
func (User) TableName() string {
	return "users"
}

func (Room) TableName() string {
	return "rooms"
}

func (Message) TableName() string {
	return "messages"
}

func (UserRoom) TableName() string {
	return "user_rooms"
}

func (PrivateMessage) TableName() string {
	return "private_messages"
}

func (UserSession) TableName() string {
	return "user_sessions"
}

func (ActivityLog) TableName() string {
	return "activity_logs"
}
