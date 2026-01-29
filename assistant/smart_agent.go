package assistant

import (
	"context"
	"fmt"
	"nofx/logger"
	"nofx/manager"
	"nofx/mcp"
	"nofx/store"
	"strings"
	"time"
)

// SmartAgent is an enhanced AI agent with trading context awareness
type SmartAgent struct {
	*Agent
	contextBuilder *ContextBuilder
	monitor        *Monitor
	
	// Auto-inject context into prompts
	autoInjectContext bool
}

// NewSmartAgent creates a new smart trading agent
func NewSmartAgent(aiClient mcp.AIClient, config AgentConfig, tm *manager.TraderManager, st *store.Store) *SmartAgent {
	baseAgent := NewAgent(aiClient, config)
	baseAgent.SetSystemPrompt(SmartTradingPrompt())
	
	contextBuilder := NewContextBuilder(tm, st)
	monitor := NewMonitor(tm, st)
	
	return &SmartAgent{
		Agent:             baseAgent,
		contextBuilder:    contextBuilder,
		monitor:           monitor,
		autoInjectContext: true,
	}
}

// SetAutoInjectContext enables/disables automatic context injection
func (sa *SmartAgent) SetAutoInjectContext(enabled bool) {
	sa.autoInjectContext = enabled
}

// StartMonitor starts the background monitor
func (sa *SmartAgent) StartMonitor() {
	sa.monitor.Start()
}

// StopMonitor stops the background monitor
func (sa *SmartAgent) StopMonitor() {
	sa.monitor.Stop()
}

// OnAlert registers an alert callback
func (sa *SmartAgent) OnAlert(callback func(Alert)) {
	sa.monitor.OnAlert(callback)
}

// Chat processes a message with smart context injection
func (sa *SmartAgent) Chat(ctx context.Context, sessionID string, userMessage string) (*AgentResponse, error) {
	session := sa.GetSession(sessionID)
	
	// Add user message to history
	session.AddMessage(Message{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	})
	
	// Build system prompt with tools
	systemPrompt := sa.buildSmartSystemPrompt()
	
	// Build conversation prompt with context injection
	conversationPrompt := sa.buildSmartConversationPrompt(session, userMessage)
	
	// Agent loop
	var finalResponse string
	toolCallCount := 0
	
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		
		if toolCallCount >= sa.config.MaxToolCalls {
			logger.Warnf("⚠️ Max tool calls reached (%d)", sa.config.MaxToolCalls)
			break
		}
		
		response, err := sa.aiClient.CallWithMessages(systemPrompt, conversationPrompt)
		if err != nil {
			return nil, fmt.Errorf("AI call failed: %w", err)
		}
		
		toolCalls, textResponse, err := sa.parseResponse(response)
		if err != nil {
			finalResponse = response
			break
		}
		
		if len(toolCalls) == 0 {
			finalResponse = textResponse
			break
		}
		
		// Execute tool calls
		toolResults := sa.executeToolCalls(ctx, toolCalls)
		toolCallCount += len(toolCalls)
		
		// Add results to conversation
		conversationPrompt += fmt.Sprintf("\n\nAssistant called tools:\n%s\n\nTool results:\n%s\n\nBased on the tool results, provide a helpful response:",
			formatToolCalls(toolCalls),
			formatToolResults(toolResults))
		
		if textResponse != "" {
			finalResponse = textResponse
		}
	}
	
	// Add response to history
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

// buildSmartSystemPrompt builds system prompt with tools
func (sa *SmartAgent) buildSmartSystemPrompt() string {
	sa.toolsLock.RLock()
	defer sa.toolsLock.RUnlock()
	
	var toolDefs []string
	for _, tool := range sa.tools {
		toolDef := fmt.Sprintf(`- **%s**: %s
  Parameters: %s`, tool.Name(), tool.Description(), tool.ParameterSchema())
		toolDefs = append(toolDefs, toolDef)
	}
	
	toolsSection := ""
	if len(toolDefs) > 0 {
		toolsSection = fmt.Sprintf(`

## 可用工具

调用工具时，使用以下 JSON 格式:
{"tool_calls": [{"name": "工具名", "arguments": {"参数": "值"}}]}

收到工具结果后，用自然语言回复用户。

可用工具:
%s
`, strings.Join(toolDefs, "\n"))
	}
	
	return sa.systemPrompt + toolsSection
}

// buildSmartConversationPrompt builds conversation with context injection
func (sa *SmartAgent) buildSmartConversationPrompt(session *Session, currentMessage string) string {
	var sb strings.Builder
	
	// Inject current trading context if enabled
	if sa.autoInjectContext {
		tradingCtx := sa.contextBuilder.BuildContext()
		sb.WriteString(tradingCtx.FormatContextForPrompt())
	}
	
	// Add conversation history
	messages := session.GetMessages()
	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("\n%s: %s\n", strings.Title(msg.Role), msg.Content))
	}
	
	return sb.String()
}

// QuickStatus returns a quick status summary
func (sa *SmartAgent) QuickStatus() string {
	ctx := sa.contextBuilder.BuildContext()
	
	var sb strings.Builder
	sb.WriteString("📊 **交易状态概览**\n\n")
	
	sb.WriteString(fmt.Sprintf("💰 总权益: $%.2f\n", ctx.TotalEquity))
	sb.WriteString(fmt.Sprintf("💵 可用余额: $%.2f\n", ctx.AvailableBalance))
	
	if ctx.UnrealizedPnL >= 0 {
		sb.WriteString(fmt.Sprintf("📈 未实现盈亏: 🟢 +$%.2f\n", ctx.UnrealizedPnL))
	} else {
		sb.WriteString(fmt.Sprintf("📉 未实现盈亏: 🔴 $%.2f\n", ctx.UnrealizedPnL))
	}
	
	sb.WriteString(fmt.Sprintf("📍 持仓数: %d\n", len(ctx.Positions)))
	sb.WriteString(fmt.Sprintf("🤖 运行交易员: %d\n", len(ctx.ActiveTraders)))
	
	if len(ctx.Alerts) > 0 {
		sb.WriteString("\n⚠️ **警报**\n")
		for _, alert := range ctx.Alerts {
			sb.WriteString(fmt.Sprintf("- %s\n", alert.Message))
		}
	}
	
	return sb.String()
}

// GetTradingContext returns current trading context
func (sa *SmartAgent) GetTradingContext() *TradingContext {
	return sa.contextBuilder.BuildContext()
}
