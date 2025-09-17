package controller

import (
	"bytes"
	"io"
	"live-chatter/internal/service"
	"live-chatter/pkg/model"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	AuthService service.AuthService
}

func NewAuthController(authService service.AuthService) *AuthController {
	return &AuthController{AuthService: authService}
}

type registerRequest struct {
	Username  string `json:"username" binding:"required,min=3,max=50"`
	Email     string `json:"email" binding:"omitempty,email,max=254"`
	Password  string `json:"password" binding:"required,min=8,max=128"`
	FirstName string `json:"first_name" binding:"omitempty,max=100"`
	LastName  string `json:"last_name" binding:"omitempty,max=100"`
}

func (ac *AuthController) Register(c *gin.Context) {
	var req registerRequest

	// Read raw body once
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[Register] Failed to read body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "details": err.Error()})
		return
	}
	log.Printf("[Register] Raw payload: %s", string(body))

	// Recreate body for binding
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	// Bind into struct
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Register] Binding into struct failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "details": err.Error()})
		return
	}
	log.Printf("[Register] Parsed request: %+v", req)

	user := model.User{
		Username:  req.Username,
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	}

	if err := ac.AuthService.Register(&user); err != nil {
		log.Printf("[Register] Service error: %v", err)
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[Register] Success: user %s registered", user.Username)
	c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully"})
}

func (ac *AuthController) Login(c *gin.Context) {
	var creds struct {
		Email    string `json:"email"`
		AuthHash string `json:"authhash"`
	}
	if err := c.ShouldBindJSON(&creds); err != nil {
		log.Printf("[Login] Invalid input: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "details": err.Error()})
		return
	}
	log.Printf("[Login] Payload: %+v", creds)

	user, err := ac.AuthService.Login(creds.Email, creds.AuthHash)
	if err != nil {
		log.Printf("[Login] Auth failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[Login] Success: %+v", user)
	c.JSON(http.StatusOK, user)
}

func (ac *AuthController) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Refresh] Invalid input: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "details": err.Error()})
		return
	}
	log.Printf("[Refresh] Payload: %+v", req)

	newTokens, err := ac.AuthService.RefreshTokens(req.RefreshToken)
	if err != nil {
		log.Printf("[Refresh] Token refresh failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[Refresh] Success: %+v", newTokens)
	c.JSON(http.StatusOK, newTokens)
}
