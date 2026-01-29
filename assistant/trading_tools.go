package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/manager"
	"nofx/store"
)

// TradingTools provides all trading-related tools for the AI agent
type TradingTools struct {
	traderManager *manager.TraderManager
	store         *store.Store
}

// NewTradingTools creates trading tools with access to NOFX core
func NewTradingTools(tm *manager.TraderManager, st *store.Store) *TradingTools {
	return &TradingTools{
		traderManager: tm,
		store:         st,
	}
}

// GetAllTools returns all trading tools
func (t *TradingTools) GetAllTools() []Tool {
	return []Tool{
		t.GetBalanceTool(),
		t.GetPositionsTool(),
		t.ListTradersTool(),
		t.GetTraderStatusTool(),
		t.StartTraderTool(),
		t.StopTraderTool(),
		t.GetMarketPriceTool(),
		t.OpenLongTool(),
		t.OpenShortTool(),
		t.ClosePositionTool(),
		t.ListStrategiesTool(),
		t.ListExchangesTool(),
		t.ListAIModelsTool(),
	}
}

// ==================== Query Tools ====================

// GetBalanceTool returns the get_balance tool
func (t *TradingTools) GetBalanceTool() Tool {
	return NewTool(
		"get_balance",
		"Get account balance for a trader. Returns available balance, total equity, and margin info.",
		`{"trader_id": "string (required) - The trader ID to query"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				TraderID string `json:"trader_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			trader, err := t.traderManager.GetTrader(params.TraderID)
			if err != nil {
				return nil, fmt.Errorf("trader not found: %w", err)
			}

			balance, err := trader.GetAccountInfo()
			if err != nil {
				return nil, fmt.Errorf("failed to get balance: %w", err)
			}

			return balance, nil
		},
	)
}

// GetPositionsTool returns the get_positions tool
func (t *TradingTools) GetPositionsTool() Tool {
	return NewTool(
		"get_positions",
		"Get all open positions for a trader. Returns symbol, side, size, entry price, unrealized P&L.",
		`{"trader_id": "string (required) - The trader ID to query"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				TraderID string `json:"trader_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			trader, err := t.traderManager.GetTrader(params.TraderID)
			if err != nil {
				return nil, fmt.Errorf("trader not found: %w", err)
			}

			positions, err := trader.GetPositions()
			if err != nil {
				return nil, fmt.Errorf("failed to get positions: %w", err)
			}

			return positions, nil
		},
	)
}

// ListTradersTool returns the list_traders tool
func (t *TradingTools) ListTradersTool() Tool {
	return NewTool(
		"list_traders",
		"List all configured AI traders with their status (running/stopped), exchange, AI model, and performance.",
		`{}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			traders, err := t.store.Trader().List("default")
			if err != nil {
				return nil, fmt.Errorf("failed to list traders: %w", err)
			}

			var result []map[string]interface{}
			for _, tr := range traders {
				traderInfo := map[string]interface{}{
					"id":          tr.ID,
					"name":        tr.Name,
					"is_running":  tr.IsRunning,
					"ai_model_id": tr.AIModelID,
					"exchange_id": tr.ExchangeID,
					"strategy_id": tr.StrategyID,
					"created_at":  tr.CreatedAt,
				}

				// Try to get live status if trader is running
				if liveTrader, err := t.traderManager.GetTrader(tr.ID); err == nil {
					status := liveTrader.GetStatus()
					traderInfo["live_status"] = status
				}

				result = append(result, traderInfo)
			}

			return result, nil
		},
	)
}

// GetTraderStatusTool returns detailed status of a specific trader
func (t *TradingTools) GetTraderStatusTool() Tool {
	return NewTool(
		"get_trader_status",
		"Get detailed status of a specific trader including current positions, recent trades, and performance metrics.",
		`{"trader_id": "string (required) - The trader ID to query"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				TraderID string `json:"trader_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			// Get trader config from store
			traderConfig, err := t.store.Trader().GetByID(params.TraderID)
			if err != nil {
				return nil, fmt.Errorf("trader not found: %w", err)
			}

			result := map[string]interface{}{
				"id":          traderConfig.ID,
				"name":        traderConfig.Name,
				"is_running":  traderConfig.IsRunning,
				"ai_model_id": traderConfig.AIModelID,
				"exchange_id": traderConfig.ExchangeID,
				"strategy_id": traderConfig.StrategyID,
			}

			// If trader is running, get live data
			trader, err := t.traderManager.GetTrader(params.TraderID)
			if err == nil && trader != nil {
				result["live_status"] = trader.GetStatus()
				if balance, err := trader.GetAccountInfo(); err == nil {
					result["balance"] = balance
				}
				if positions, err := trader.GetPositions(); err == nil {
					result["positions"] = positions
				}
			}

			return result, nil
		},
	)
}

// ==================== Control Tools ====================

// StartTraderTool starts an AI trader
func (t *TradingTools) StartTraderTool() Tool {
	return NewTool(
		"start_trader",
		"Start an AI trader to begin automated trading. The trader will execute trades based on its configured strategy.",
		`{"trader_id": "string (required) - The trader ID to start"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				TraderID string `json:"trader_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			// Check if already running
			existingTrader, _ := t.traderManager.GetTrader(params.TraderID)
			if existingTrader != nil {
				status := existingTrader.GetStatus()
				if isRunning, ok := status["is_running"].(bool); ok && isRunning {
					return nil, fmt.Errorf("trader is already running")
				}
				// Remove from memory to reload
				t.traderManager.RemoveTrader(params.TraderID)
			}

			// Load and start trader
			if err := t.traderManager.LoadUserTradersFromStore(t.store, "default"); err != nil {
				return nil, fmt.Errorf("failed to load trader: %w", err)
			}

			trader, err := t.traderManager.GetTrader(params.TraderID)
			if err != nil {
				return nil, fmt.Errorf("failed to get trader after load: %w", err)
			}

			// Start the trader in a goroutine
			go func() {
				if err := trader.Run(); err != nil {
					logger.Errorf("Trader %s error: %v", params.TraderID, err)
				}
			}()

			// Update status in database
			if err := t.store.Trader().UpdateStatus("default", params.TraderID, true); err != nil {
				logger.Warnf("Failed to update trader status in DB: %v", err)
			}

			return map[string]interface{}{
				"success":   true,
				"trader_id": params.TraderID,
				"message":   "Trader started successfully",
			}, nil
		},
	)
}

// StopTraderTool stops an AI trader
func (t *TradingTools) StopTraderTool() Tool {
	return NewTool(
		"stop_trader",
		"Stop an AI trader. This will halt automated trading but keep existing positions open.",
		`{"trader_id": "string (required) - The trader ID to stop"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				TraderID string `json:"trader_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			trader, err := t.traderManager.GetTrader(params.TraderID)
			if err != nil {
				return nil, fmt.Errorf("trader not found: %w", err)
			}

			// Check if running
			status := trader.GetStatus()
			if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
				return nil, fmt.Errorf("trader is already stopped")
			}

			// Stop the trader
			trader.Stop()

			// Update status in database
			if err := t.store.Trader().UpdateStatus("default", params.TraderID, false); err != nil {
				logger.Warnf("Failed to update trader status in DB: %v", err)
			}

			return map[string]interface{}{
				"success":   true,
				"trader_id": params.TraderID,
				"message":   "Trader stopped successfully",
			}, nil
		},
	)
}

// ==================== Trading Tools ====================

// GetMarketPriceTool gets current market price
func (t *TradingTools) GetMarketPriceTool() Tool {
	return NewTool(
		"get_market_price",
		"Get current market price for a trading pair from a specific trader's exchange.",
		`{"trader_id": "string (required)", "symbol": "string (required) - e.g., BTCUSDT"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				TraderID string `json:"trader_id"`
				Symbol   string `json:"symbol"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			autoTrader, err := t.traderManager.GetTrader(params.TraderID)
			if err != nil {
				return nil, fmt.Errorf("trader not found: %w", err)
			}

			// Get the underlying trader interface
			underlyingTrader := autoTrader.GetUnderlyingTrader()
			if underlyingTrader == nil {
				return nil, fmt.Errorf("underlying trader not available")
			}

			price, err := underlyingTrader.GetMarketPrice(params.Symbol)
			if err != nil {
				return nil, fmt.Errorf("failed to get price: %w", err)
			}

			return map[string]interface{}{
				"symbol": params.Symbol,
				"price":  price,
			}, nil
		},
	)
}

// OpenLongTool opens a long position
func (t *TradingTools) OpenLongTool() Tool {
	return NewTool(
		"open_long",
		"Open a long (buy) position. WARNING: This will execute a real trade!",
		`{"trader_id": "string (required)", "symbol": "string (required)", "quantity": "number (required)", "leverage": "number (optional, default 1)"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				TraderID string  `json:"trader_id"`
				Symbol   string  `json:"symbol"`
				Quantity float64 `json:"quantity"`
				Leverage int     `json:"leverage"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			if params.Leverage == 0 {
				params.Leverage = 1
			}

			autoTrader, err := t.traderManager.GetTrader(params.TraderID)
			if err != nil {
				return nil, fmt.Errorf("trader not found: %w", err)
			}

			underlyingTrader := autoTrader.GetUnderlyingTrader()
			if underlyingTrader == nil {
				return nil, fmt.Errorf("underlying trader not available")
			}

			result, err := underlyingTrader.OpenLong(params.Symbol, params.Quantity, params.Leverage)
			if err != nil {
				return nil, fmt.Errorf("failed to open long: %w", err)
			}

			return result, nil
		},
	)
}

// OpenShortTool opens a short position
func (t *TradingTools) OpenShortTool() Tool {
	return NewTool(
		"open_short",
		"Open a short (sell) position. WARNING: This will execute a real trade!",
		`{"trader_id": "string (required)", "symbol": "string (required)", "quantity": "number (required)", "leverage": "number (optional, default 1)"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				TraderID string  `json:"trader_id"`
				Symbol   string  `json:"symbol"`
				Quantity float64 `json:"quantity"`
				Leverage int     `json:"leverage"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			if params.Leverage == 0 {
				params.Leverage = 1
			}

			autoTrader, err := t.traderManager.GetTrader(params.TraderID)
			if err != nil {
				return nil, fmt.Errorf("trader not found: %w", err)
			}

			underlyingTrader := autoTrader.GetUnderlyingTrader()
			if underlyingTrader == nil {
				return nil, fmt.Errorf("underlying trader not available")
			}

			result, err := underlyingTrader.OpenShort(params.Symbol, params.Quantity, params.Leverage)
			if err != nil {
				return nil, fmt.Errorf("failed to open short: %w", err)
			}

			return result, nil
		},
	)
}

// ClosePositionTool closes a position
func (t *TradingTools) ClosePositionTool() Tool {
	return NewTool(
		"close_position",
		"Close an existing position (long or short). WARNING: This will execute a real trade!",
		`{"trader_id": "string (required)", "symbol": "string (required)", "side": "string (required) - 'long' or 'short'", "quantity": "number (optional) - leave empty to close all"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				TraderID string  `json:"trader_id"`
				Symbol   string  `json:"symbol"`
				Side     string  `json:"side"`
				Quantity float64 `json:"quantity"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			autoTrader, err := t.traderManager.GetTrader(params.TraderID)
			if err != nil {
				return nil, fmt.Errorf("trader not found: %w", err)
			}

			underlyingTrader := autoTrader.GetUnderlyingTrader()
			if underlyingTrader == nil {
				return nil, fmt.Errorf("underlying trader not available")
			}

			var result map[string]interface{}

			if params.Side == "long" {
				result, err = underlyingTrader.CloseLong(params.Symbol, params.Quantity)
			} else if params.Side == "short" {
				result, err = underlyingTrader.CloseShort(params.Symbol, params.Quantity)
			} else {
				return nil, fmt.Errorf("invalid side: %s (must be 'long' or 'short')", params.Side)
			}

			if err != nil {
				return nil, fmt.Errorf("failed to close position: %w", err)
			}

			return result, nil
		},
	)
}

// ==================== Config Tools ====================

// ListStrategiesTool lists all strategies
func (t *TradingTools) ListStrategiesTool() Tool {
	return NewTool(
		"list_strategies",
		"List all trading strategies configured in the system.",
		`{}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			strategies, err := t.store.Strategy().List("default")
			if err != nil {
				return nil, fmt.Errorf("failed to list strategies: %w", err)
			}
			return strategies, nil
		},
	)
}

// ListExchangesTool lists all exchange configurations
func (t *TradingTools) ListExchangesTool() Tool {
	return NewTool(
		"list_exchanges",
		"List all configured exchanges (without showing API keys).",
		`{}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			exchanges, err := t.store.Exchange().List("default")
			if err != nil {
				return nil, fmt.Errorf("failed to list exchanges: %w", err)
			}

			// Remove sensitive data
			var result []map[string]interface{}
			for _, ex := range exchanges {
				result = append(result, map[string]interface{}{
					"id":            ex.ID,
					"name":          ex.Name,
					"exchange_type": ex.ExchangeType,
					"type":          ex.Type,
					"enabled":       ex.Enabled,
				})
			}
			return result, nil
		},
	)
}

// ListAIModelsTool lists all AI model configurations
func (t *TradingTools) ListAIModelsTool() Tool {
	return NewTool(
		"list_ai_models",
		"List all configured AI models (without showing API keys).",
		`{}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			models, err := t.store.AIModel().List("default")
			if err != nil {
				return nil, fmt.Errorf("failed to list AI models: %w", err)
			}

			// Remove sensitive data
			var result []map[string]interface{}
			for _, m := range models {
				result = append(result, map[string]interface{}{
					"id":               m.ID,
					"name":             m.Name,
					"provider":         m.Provider,
					"custom_model":     m.CustomModelName,
					"enabled":          m.Enabled,
				})
			}
			return result, nil
		},
	)
}
