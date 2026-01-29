// Package assistant implements the AI Agent runtime with tool calling capabilities
// Inspired by moltbot's agent architecture, specialized for trading
package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/mcp"
	"strings"
	"sync"
	"time"
)

// Agent represents an AI assistant with tool-calling capabilities
type Agent struct {
	// AI client for LLM calls
	aiClient mcp.AIClient

	// Tool registry
	tools     map[string]Tool
	toolsLock sync.RWMutex

	// Session/memory management
	sessions     map[string]*Session
	sessionsLock sync.RWMutex

	// Configuration
	config AgentConfig

	// System prompt
	systemPrompt string
}

// AgentConfig holds agent configuration
type AgentConfig struct {
	// Max tool calls per turn (prevent infinite loops)
	MaxToolCalls int `json:"max_tool_calls"`

	// Max conversation history to keep
	MaxHistoryMessages int `json:"max_history_messages"`

	// Timeout for single AI call
	AITimeout time.Duration `json:"ai_timeout"`

	// Model to use
	Model string `json:"model"`
}

// DefaultAgentConfig returns sensible defaults
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		MaxToolCalls:       10,
		MaxHistoryMessages: 50,
		AITimeout:          120 * time.Second,
		Model:              "deepseek-chat",
	}
}

// NewAgent creates a new AI agent
func NewAgent(aiClient mcp.AIClient, config AgentConfig) *Agent {
	agent := &Agent{
		aiClient: aiClient,
		tools:    make(map[string]Tool),
		sessions: make(map[string]*Session),
		config:   config,
	}

	// Set default system prompt
	agent.systemPrompt = DefaultTradingSystemPrompt()

	return agent
}

// RegisterTool adds a tool to the agent's toolkit
func (a *Agent) RegisterTool(tool Tool) {
	a.toolsLock.Lock()
	defer a.toolsLock.Unlock()
	a.tools[tool.Name()] = tool
	logger.Infof("🔧 Registered tool: %s", tool.Name())
}

// RegisterTools adds multiple tools
func (a *Agent) RegisterTools(tools ...Tool) {
	for _, tool := range tools {
		a.RegisterTool(tool)
	}
}

// SetSystemPrompt sets the agent's system prompt
func (a *Agent) SetSystemPrompt(prompt string) {
	a.systemPrompt = prompt
}

// GetSession returns or creates a session for the given ID
func (a *Agent) GetSession(sessionID string) *Session {
	a.sessionsLock.Lock()
	defer a.sessionsLock.Unlock()

	if session, ok := a.sessions[sessionID]; ok {
		return session
	}

	session := NewSession(sessionID, a.config.MaxHistoryMessages)
	a.sessions[sessionID] = session
	return session
}

// Chat processes a user message and returns the agent's response
// This is the main entry point for the agent loop
func (a *Agent) Chat(ctx context.Context, sessionID string, userMessage string) (*AgentResponse, error) {
	session := a.GetSession(sessionID)

	// Add user message to history
	session.AddMessage(Message{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	})

	// Build the full prompt with tools
	systemPrompt := a.buildSystemPromptWithTools()
	conversationPrompt := a.buildConversationPrompt(session)

	// Agent loop - keep calling AI until it's done or max iterations
	var finalResponse string
	toolCallCount := 0

	for {
		// Check context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Check max tool calls
		if toolCallCount >= a.config.MaxToolCalls {
			logger.Warnf("⚠️ Max tool calls reached (%d), stopping agent loop", a.config.MaxToolCalls)
			break
		}

		// Call AI
		response, err := a.aiClient.CallWithMessages(systemPrompt, conversationPrompt)
		if err != nil {
			return nil, fmt.Errorf("AI call failed: %w", err)
		}

		// Parse response for tool calls
		toolCalls, textResponse, err := a.parseResponse(response)
		if err != nil {
			// If parsing fails, treat entire response as text
			finalResponse = response
			break
		}

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			finalResponse = textResponse
			break
		}

		// Execute tool calls
		toolResults := a.executeToolCalls(ctx, toolCalls)
		toolCallCount += len(toolCalls)

		// Add tool calls and results to conversation for next iteration
		conversationPrompt += fmt.Sprintf("\n\nAssistant called tools:\n%s\n\nTool results:\n%s\n\nBased on the tool results, please provide your response to the user:",
			formatToolCalls(toolCalls),
			formatToolResults(toolResults))

		// If there's also a text response, capture it
		if textResponse != "" {
			finalResponse = textResponse
		}
	}

	// Add assistant response to history
	session.AddMessage(Message{
		Role:      "assistant",
		Content:   finalResponse,
		Timestamp: time.Now(),
	})

	return &AgentResponse{
		Text:      finalResponse,
		SessionID: sessionID,
	}, nil
}

// buildSystemPromptWithTools creates the system prompt including tool definitions
func (a *Agent) buildSystemPromptWithTools() string {
	a.toolsLock.RLock()
	defer a.toolsLock.RUnlock()

	var toolDefs []string
	for _, tool := range a.tools {
		toolDef := fmt.Sprintf(`- **%s**: %s
  Parameters: %s`, tool.Name(), tool.Description(), tool.ParameterSchema())
		toolDefs = append(toolDefs, toolDef)
	}

	toolsSection := ""
	if len(toolDefs) > 0 {
		toolsSection = fmt.Sprintf(`

## Available Tools

You can call tools by responding with JSON in this format:
{"tool_calls": [{"name": "tool_name", "arguments": {"param": "value"}}]}

After receiving tool results, provide a natural language response to the user.

Tools:
%s
`, strings.Join(toolDefs, "\n"))
	}

	return a.systemPrompt + toolsSection
}

// buildConversationPrompt builds the conversation history as a prompt
func (a *Agent) buildConversationPrompt(session *Session) string {
	messages := session.GetMessages()
	var parts []string

	for _, msg := range messages {
		parts = append(parts, fmt.Sprintf("%s: %s", strings.Title(msg.Role), msg.Content))
	}

	return strings.Join(parts, "\n\n")
}

// parseResponse extracts tool calls and text from AI response
func (a *Agent) parseResponse(response string) ([]ToolCall, string, error) {
	// Try to find JSON tool calls in response
	// Look for {"tool_calls": [...]} pattern
	
	var toolCalls []ToolCall
	textResponse := response

	// Try to parse as JSON
	if strings.Contains(response, "tool_calls") {
		// Find JSON block
		start := strings.Index(response, "{")
		end := strings.LastIndex(response, "}")
		
		if start >= 0 && end > start {
			jsonStr := response[start : end+1]
			
			var parsed struct {
				ToolCalls []struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"tool_calls"`
			}
			
			if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
				for _, tc := range parsed.ToolCalls {
					toolCalls = append(toolCalls, ToolCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					})
				}
				
				// Extract text before/after JSON
				textResponse = strings.TrimSpace(response[:start] + response[end+1:])
			}
		}
	}

	return toolCalls, textResponse, nil
}

// executeToolCalls runs the requested tools
func (a *Agent) executeToolCalls(ctx context.Context, calls []ToolCall) []ToolResult {
	a.toolsLock.RLock()
	defer a.toolsLock.RUnlock()

	var results []ToolResult

	for _, call := range calls {
		tool, ok := a.tools[call.Name]
		if !ok {
			results = append(results, ToolResult{
				Name:  call.Name,
				Error: fmt.Sprintf("unknown tool: %s", call.Name),
			})
			continue
		}

		logger.Infof("🔧 Executing tool: %s", call.Name)
		
		result, err := tool.Execute(ctx, call.Arguments)
		if err != nil {
			logger.Errorf("❌ Tool %s failed: %v", call.Name, err)
			results = append(results, ToolResult{
				Name:  call.Name,
				Error: err.Error(),
			})
		} else {
			logger.Infof("✅ Tool %s completed", call.Name)
			results = append(results, ToolResult{
				Name:   call.Name,
				Result: result,
			})
		}
	}

	return results
}

// ToolCall represents a tool invocation request from the AI
type ToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Name   string      `json:"name"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// AgentResponse is the final response from the agent
type AgentResponse struct {
	Text      string `json:"text"`
	SessionID string `json:"session_id"`
}

func formatToolCalls(calls []ToolCall) string {
	var parts []string
	for _, c := range calls {
		parts = append(parts, fmt.Sprintf("- %s(%s)", c.Name, string(c.Arguments)))
	}
	return strings.Join(parts, "\n")
}

func formatToolResults(results []ToolResult) string {
	var parts []string
	for _, r := range results {
		if r.Error != "" {
			parts = append(parts, fmt.Sprintf("- %s: ERROR: %s", r.Name, r.Error))
		} else {
			resultJSON, _ := json.Marshal(r.Result)
			parts = append(parts, fmt.Sprintf("- %s: %s", r.Name, string(resultJSON)))
		}
	}
	return strings.Join(parts, "\n")
}
