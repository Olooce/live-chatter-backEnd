
# LiveChatter

**LiveChatter** is a learning project to explore real-time chat with Go, Gin, WebSockets, and GORM. It demonstrates sending and receiving messages in real-time, connection management, and basic logging.

> Messages are stored in plain text - this project is **for learning purposes only** and not secure for production.
> 
> Exhibits poor connection management, maxing out at just over 111K database operations per second - a suboptimal performance for a Go application.

**Tech stack:**

* Go (Gin framework)
* WebSockets for real-time messaging
* GORM for database persistence
* Custom logging utilities

**Purpose:** Learn how real-time chat works while practicing Go, WebSockets, and server-side logging.

---