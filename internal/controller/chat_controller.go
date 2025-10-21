package controller

import (
	Log "live-chatter/pkg/logger"
	"net/http"
	"strconv"
	"time"

	"live-chatter/internal/service"
	"live-chatter/pkg/model"

	"github.com/gin-gonic/gin"
)

type ChatController struct {
	ChatService service.ChatService
}

func NewChatController(chatService service.ChatService) *ChatController {
	return &ChatController{ChatService: chatService}
}

// GetRooms returns all available chat rooms
func (cc *ChatController) GetRooms(c *gin.Context) {
	rooms, err := cc.ChatService.GetAllRooms()
	if err != nil {
		Log.Error("Error getting rooms: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rooms"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rooms": rooms})
}

// CreateRoom creates a new chat room
func (cc *ChatController) CreateRoom(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required,min=1,max=50"`
		Description string `json:"description" binding:"omitempty,max=255"`
		Type        string `json:"type" binding:"omitempty,oneof=public private"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		Log.Error("Error binding json: ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		Log.Error("Required User ID not found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDUint := userID.(uint)
	room := &model.Room{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		CreatedBy:   userIDUint,
	}

	if room.Type == "" {
		room.Type = "public"
	}

	createdRoom, err := cc.ChatService.CreateRoom(room)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create room"})
		Log.Error("Error creating Room", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"room": createdRoom})
}

// GetRoomMessages returns messages for a specific room with pagination
func (cc *ChatController) GetRoomMessages(c *gin.Context) {
	roomID := c.Param("roomId")
	if roomID == "" {
		Log.Error("Invalid roomId")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Room ID is required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	beforeStr := c.Query("before")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		Log.Warn("Invalid offset: ", err)
		offset = 0
	}

	var before *time.Time
	if beforeStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			before = &parsedTime
		}
	}

	messages, err := cc.ChatService.GetRoomMessages(roomID, limit, offset, before)
	if err != nil {
		Log.Error("Error getting room [%s] messages: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"limit":    limit,
		"offset":   offset,
	})
}

// JoinRoom adds a user to a room
func (cc *ChatController) JoinRoom(c *gin.Context) {
	roomID := c.Param("roomId")
	if roomID == "" {
		Log.Error("Room ID is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Room ID is required"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		Log.Error("Required User ID not found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := cc.ChatService.JoinRoom(roomID, userID.(uint))
	if err != nil {
		Log.Error("Error joining room: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully joined room"})
}

// LeaveRoom removes a user from a room
func (cc *ChatController) LeaveRoom(c *gin.Context) {
	roomID := c.Param("roomId")
	if roomID == "" {
		Log.Error("Room ID is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Room ID is required"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		Log.Error("Required User ID not found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDUint := userID.(uint)
	err := cc.ChatService.LeaveRoom(roomID, userIDUint)
	if err != nil {
		Log.Error("Error leaving room: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully left room"})
}

func (cc *ChatController) GetUserRooms(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Log.Error("Required User ID not found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDUint := userID.(uint)
	rooms, err := cc.ChatService.GetUserRooms(userIDUint)
	if err != nil {
		Log.Error("Error getting rooms: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user rooms"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rooms": rooms})
}

// GetOnlineUsers returns currently online users
func (cc *ChatController) GetOnlineUsers(c *gin.Context) {
	users, err := cc.ChatService.GetOnlineUsers()
	if err != nil {
		Log.Error("Error getting online users: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch online users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"count": len(users),
	})
}

// SearchMessages searches for messages containing specific text
func (cc *ChatController) SearchMessages(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		Log.Error("Query is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	roomID := c.Query("room_id")
	limitStr := c.DefaultQuery("limit", "20")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 50 {
		Log.Warn("Invalid limit: ", err)
		limit = 20
	}

	messages, err := cc.ChatService.SearchMessages(query, roomID, limit)
	if err != nil {
		Log.Error("Error searching messages: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"query":    query,
		"count":    len(messages),
	})
}
