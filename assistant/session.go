package assistant

import (
	"sync"
	"time"
)

// Message represents a single message in conversation
type Message struct {
	Role      string    `json:"role"` // "user", "assistant", "system", "tool"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	
	// For tool messages
	ToolName   string      `json:"tool_name,omitempty"`
	ToolResult interface{} `json:"tool_result,omitempty"`
}

// Session represents a conversation session with memory
type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	// User info
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	Platform string `json:"platform"` // "telegram", "web", etc.
	
	// Conversation history
	messages    []Message
	maxMessages int
	mu          sync.RWMutex
	
	// Custom metadata
	Metadata map[string]interface{} `json:"metadata"`
}

// NewSession creates a new conversation session
func NewSession(id string, maxMessages int) *Session {
	return &Session{
		ID:          id,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		messages:    make([]Message, 0),
		maxMessages: maxMessages,
		Metadata:    make(map[string]interface{}),
	}
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, msg)
	s.UpdatedAt = time.Now()

	// Trim old messages if exceeding max
	if len(s.messages) > s.maxMessages {
		// Keep the most recent messages
		s.messages = s.messages[len(s.messages)-s.maxMessages:]
	}
}

// GetMessages returns a copy of all messages
func (s *Session) GetMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Message, len(s.messages))
	copy(result, s.messages)
	return result
}

// GetRecentMessages returns the N most recent messages
func (s *Session) GetRecentMessages(n int) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n >= len(s.messages) {
		result := make([]Message, len(s.messages))
		copy(result, s.messages)
		return result
	}

	result := make([]Message, n)
	copy(result, s.messages[len(s.messages)-n:])
	return result
}

// Clear removes all messages from the session
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = make([]Message, 0)
	s.UpdatedAt = time.Now()
}

// SetUserInfo sets user information
func (s *Session) SetUserInfo(userID, userName, platform string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UserID = userID
	s.UserName = userName
	s.Platform = platform
}

// SetMetadata sets a metadata value
func (s *Session) SetMetadata(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Metadata[key] = value
}

// GetMetadata gets a metadata value
func (s *Session) GetMetadata(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.Metadata[key]
	return v, ok
}
