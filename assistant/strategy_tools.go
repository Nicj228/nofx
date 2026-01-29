package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"nofx/store"
)

// StrategyTools provides strategy management tools for the AI agent
type StrategyTools struct {
	store           *store.Store
	strategyBuilder *StrategyBuilder
	strategies      map[string]*SmartStrategy // In-memory strategy cache
}

// NewStrategyTools creates strategy tools
func NewStrategyTools(st *store.Store) *StrategyTools {
	return &StrategyTools{
		store:           st,
		strategyBuilder: NewStrategyBuilder(st),
		strategies:      make(map[string]*SmartStrategy),
	}
}

// GetAllTools returns all strategy tools
func (st *StrategyTools) GetAllTools() []Tool {
	return []Tool{
		st.CreateStrategyTool(),
		st.CreateGridStrategyTool(),
		st.CreateDCAStrategyTool(),
		st.CreateTrendStrategyTool(),
		st.ListSmartStrategiesTool(),
		st.GetStrategyDetailsTool(),
		st.UpdateStrategyTool(),
		st.ActivateStrategyTool(),
		st.DeactivateStrategyTool(),
		st.DeleteStrategyTool(),
		st.GetStrategyTemplates(),
	}
}

// CreateStrategyTool creates a strategy from natural language
func (st *StrategyTools) CreateStrategyTool() Tool {
	return NewTool(
		"create_strategy",
		`Create a new trading strategy from natural language description. 
Examples: 
- "当RSI低于30时买入BTC，RSI高于70时卖出"
- "每天定投100美元ETH"
- "BTC在5万到6万之间做网格交易"`,
		`{
			"name": "string (required) - Strategy name",
			"description": "string (required) - Natural language description of the strategy",
			"symbols": "array (optional) - Trading pairs, e.g., [\"BTCUSDT\", \"ETHUSDT\"]",
			"take_profit": "number (optional) - Take profit percentage",
			"stop_loss": "number (optional) - Stop loss percentage",
			"leverage": "number (optional) - Leverage to use (default: 3)",
			"max_positions": "number (optional) - Max concurrent positions (default: 5)"
		}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				Name         string   `json:"name"`
				Description  string   `json:"description"`
				Symbols      []string `json:"symbols"`
				TakeProfit   *float64 `json:"take_profit"`
				StopLoss     *float64 `json:"stop_loss"`
				Leverage     int      `json:"leverage"`
				MaxPositions int      `json:"max_positions"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			if params.Description == "" {
				return nil, fmt.Errorf("strategy description is required")
			}

			strategy, err := st.strategyBuilder.CreateFromNaturalLanguage(params.Description, "default")
			if err != nil {
				return nil, err
			}

			// Apply user customizations
			if params.Name != "" {
				strategy.Name = params.Name
			}
			if len(params.Symbols) > 0 {
				strategy.Symbols = params.Symbols
				strategy.SymbolMode = "static"
			}
			if params.TakeProfit != nil {
				strategy.TakeProfit = params.TakeProfit
			}
			if params.StopLoss != nil {
				strategy.StopLoss = params.StopLoss
			}
			if params.Leverage > 0 {
				strategy.LeverageConfig.DefaultLeverage = params.Leverage
			}
			if params.MaxPositions > 0 {
				strategy.MaxPositions = params.MaxPositions
			}

			// Store in memory
			st.strategies[strategy.ID] = strategy

			return map[string]interface{}{
				"success":  true,
				"strategy": strategy,
				"message":  fmt.Sprintf("策略 '%s' (ID: %s) 创建成功！使用 activate_strategy 激活它。", strategy.Name, strategy.ID),
			}, nil
		},
	)
}

// CreateGridStrategyTool creates a grid trading strategy
func (st *StrategyTools) CreateGridStrategyTool() Tool {
	return NewTool(
		"create_grid_strategy",
		"Create a grid trading strategy. Grid trading places buy and sell orders at predetermined price levels.",
		`{
			"symbol": "string (required) - Trading pair, e.g., BTCUSDT",
			"lower_price": "number (required) - Lower price bound",
			"upper_price": "number (required) - Upper price bound",
			"grid_count": "number (required) - Number of grids (10-100)",
			"amount_per_grid": "number (required) - USDT amount per grid"
		}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				Symbol        string  `json:"symbol"`
				LowerPrice    float64 `json:"lower_price"`
				UpperPrice    float64 `json:"upper_price"`
				GridCount     int     `json:"grid_count"`
				AmountPerGrid float64 `json:"amount_per_grid"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			if params.LowerPrice >= params.UpperPrice {
				return nil, fmt.Errorf("lower_price must be less than upper_price")
			}
			if params.GridCount < 2 || params.GridCount > 100 {
				return nil, fmt.Errorf("grid_count must be between 2 and 100")
			}

			strategy := st.strategyBuilder.CreateGridStrategy(
				params.Symbol, params.LowerPrice, params.UpperPrice,
				params.GridCount, params.AmountPerGrid,
			)
			st.strategies[strategy.ID] = strategy

			gridSize := (params.UpperPrice - params.LowerPrice) / float64(params.GridCount)
			totalInvestment := params.AmountPerGrid * float64(params.GridCount)

			return map[string]interface{}{
				"success":  true,
				"strategy": strategy,
				"details": map[string]interface{}{
					"grid_size":        gridSize,
					"total_investment": totalInvestment,
					"profit_per_grid":  (gridSize / params.LowerPrice) * 100,
				},
				"message": fmt.Sprintf("网格策略创建成功！\n价格区间: %.2f - %.2f\n网格数: %d\n每格间距: %.2f\n总投资: $%.2f",
					params.LowerPrice, params.UpperPrice, params.GridCount, gridSize, totalInvestment),
			}, nil
		},
	)
}

// CreateDCAStrategyTool creates a DCA strategy
func (st *StrategyTools) CreateDCAStrategyTool() Tool {
	return NewTool(
		"create_dca_strategy",
		"Create a Dollar Cost Averaging (DCA) strategy. Automatically buy at regular intervals.",
		`{
			"symbol": "string (required) - Trading pair, e.g., BTCUSDT",
			"interval_minutes": "number (required) - Buy interval in minutes (min: 5)",
			"amount_per_buy": "number (required) - USDT amount per purchase",
			"max_buys": "number (optional) - Maximum number of buys (default: unlimited)"
		}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				Symbol          string  `json:"symbol"`
				IntervalMinutes int     `json:"interval_minutes"`
				AmountPerBuy    float64 `json:"amount_per_buy"`
				MaxBuys         int     `json:"max_buys"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			if params.IntervalMinutes < 5 {
				return nil, fmt.Errorf("interval must be at least 5 minutes")
			}
			if params.MaxBuys == 0 {
				params.MaxBuys = 1000 // Effectively unlimited
			}

			strategy := st.strategyBuilder.CreateDCAStrategy(
				params.Symbol, params.IntervalMinutes, params.AmountPerBuy, params.MaxBuys,
			)
			st.strategies[strategy.ID] = strategy

			return map[string]interface{}{
				"success":  true,
				"strategy": strategy,
				"message": fmt.Sprintf("DCA策略创建成功！\n币种: %s\n定投间隔: %d分钟\n每次金额: $%.2f\n最大次数: %d",
					params.Symbol, params.IntervalMinutes, params.AmountPerBuy, params.MaxBuys),
			}, nil
		},
	)
}

// CreateTrendStrategyTool creates a trend following strategy
func (st *StrategyTools) CreateTrendStrategyTool() Tool {
	return NewTool(
		"create_trend_strategy",
		"Create a trend following strategy using EMA crossover.",
		`{
			"symbols": "array (required) - Trading pairs",
			"ema_fast": "number (optional) - Fast EMA period (default: 9)",
			"ema_slow": "number (optional) - Slow EMA period (default: 21)",
			"leverage": "number (optional) - Leverage (default: 3)",
			"take_profit": "number (optional) - Take profit %",
			"stop_loss": "number (optional) - Stop loss %"
		}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				Symbols    []string `json:"symbols"`
				EMAFast    int      `json:"ema_fast"`
				EMASlow    int      `json:"ema_slow"`
				Leverage   int      `json:"leverage"`
				TakeProfit *float64 `json:"take_profit"`
				StopLoss   *float64 `json:"stop_loss"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			if len(params.Symbols) == 0 {
				params.Symbols = []string{"BTCUSDT", "ETHUSDT"}
			}
			if params.EMAFast == 0 {
				params.EMAFast = 9
			}
			if params.EMASlow == 0 {
				params.EMASlow = 21
			}
			if params.Leverage == 0 {
				params.Leverage = 3
			}

			strategy := st.strategyBuilder.CreateTrendStrategy(
				params.Symbols, params.EMAFast, params.EMASlow, params.Leverage,
			)
			strategy.TakeProfit = params.TakeProfit
			strategy.StopLoss = params.StopLoss
			st.strategies[strategy.ID] = strategy

			return map[string]interface{}{
				"success":  true,
				"strategy": strategy,
				"message": fmt.Sprintf("趋势策略创建成功！\nEMA %d/%d 交叉\n交易对: %v\n杠杆: %dx",
					params.EMAFast, params.EMASlow, params.Symbols, params.Leverage),
			}, nil
		},
	)
}

// ListSmartStrategiesTool lists all smart strategies
func (st *StrategyTools) ListSmartStrategiesTool() Tool {
	return NewTool(
		"list_smart_strategies",
		"List all smart strategies (both in-memory and saved).",
		`{}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var result []map[string]interface{}

			for _, s := range st.strategies {
				result = append(result, map[string]interface{}{
					"id":          s.ID,
					"name":        s.Name,
					"type":        s.Type,
					"description": s.Description,
					"is_active":   s.IsActive,
					"symbols":     s.Symbols,
					"created_at":  s.CreatedAt,
				})
			}

			// Also get strategies from store
			if dbStrategies, err := st.store.Strategy().List("default"); err == nil {
				for _, s := range dbStrategies {
					result = append(result, map[string]interface{}{
						"id":          s.ID,
						"name":        s.Name,
						"type":        "db_strategy",
						"description": s.Description,
						"is_active":   s.IsActive,
						"source":      "database",
					})
				}
			}

			if len(result) == 0 {
				return map[string]interface{}{
					"strategies": []interface{}{},
					"message":    "暂无策略。使用 create_strategy 创建一个新策略。",
				}, nil
			}

			return map[string]interface{}{
				"strategies": result,
				"count":      len(result),
			}, nil
		},
	)
}

// GetStrategyDetailsTool gets detailed strategy info
func (st *StrategyTools) GetStrategyDetailsTool() Tool {
	return NewTool(
		"get_strategy_details",
		"Get detailed information about a specific strategy.",
		`{"strategy_id": "string (required)"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				StrategyID string `json:"strategy_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			if s, ok := st.strategies[params.StrategyID]; ok {
				return map[string]interface{}{
					"strategy":    s,
					"prompt_text": StrategyToPrompt(s),
				}, nil
			}

			return nil, fmt.Errorf("strategy not found: %s", params.StrategyID)
		},
	)
}

// UpdateStrategyTool updates a strategy
func (st *StrategyTools) UpdateStrategyTool() Tool {
	return NewTool(
		"update_strategy",
		"Update an existing strategy's settings.",
		`{
			"strategy_id": "string (required)",
			"name": "string (optional)",
			"take_profit": "number (optional)",
			"stop_loss": "number (optional)",
			"leverage": "number (optional)",
			"max_positions": "number (optional)",
			"symbols": "array (optional)"
		}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				StrategyID   string   `json:"strategy_id"`
				Name         string   `json:"name"`
				TakeProfit   *float64 `json:"take_profit"`
				StopLoss     *float64 `json:"stop_loss"`
				Leverage     int      `json:"leverage"`
				MaxPositions int      `json:"max_positions"`
				Symbols      []string `json:"symbols"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			s, ok := st.strategies[params.StrategyID]
			if !ok {
				return nil, fmt.Errorf("strategy not found: %s", params.StrategyID)
			}

			if params.Name != "" {
				s.Name = params.Name
			}
			if params.TakeProfit != nil {
				s.TakeProfit = params.TakeProfit
			}
			if params.StopLoss != nil {
				s.StopLoss = params.StopLoss
			}
			if params.Leverage > 0 {
				s.LeverageConfig.DefaultLeverage = params.Leverage
			}
			if params.MaxPositions > 0 {
				s.MaxPositions = params.MaxPositions
			}
			if len(params.Symbols) > 0 {
				s.Symbols = params.Symbols
			}

			return map[string]interface{}{
				"success":  true,
				"strategy": s,
				"message":  "策略已更新",
			}, nil
		},
	)
}

// ActivateStrategyTool activates a strategy
func (st *StrategyTools) ActivateStrategyTool() Tool {
	return NewTool(
		"activate_strategy",
		"Activate a strategy to start trading. ⚠️ This will start real trading!",
		`{"strategy_id": "string (required)"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				StrategyID string `json:"strategy_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			s, ok := st.strategies[params.StrategyID]
			if !ok {
				return nil, fmt.Errorf("strategy not found: %s", params.StrategyID)
			}

			s.IsActive = true

			return map[string]interface{}{
				"success":  true,
				"message":  fmt.Sprintf("⚠️ 策略 '%s' 已激活！将开始真实交易。", s.Name),
				"strategy": s,
			}, nil
		},
	)
}

// DeactivateStrategyTool deactivates a strategy
func (st *StrategyTools) DeactivateStrategyTool() Tool {
	return NewTool(
		"deactivate_strategy",
		"Deactivate a strategy to stop trading.",
		`{"strategy_id": "string (required)"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				StrategyID string `json:"strategy_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			s, ok := st.strategies[params.StrategyID]
			if !ok {
				return nil, fmt.Errorf("strategy not found: %s", params.StrategyID)
			}

			s.IsActive = false

			return map[string]interface{}{
				"success": true,
				"message": fmt.Sprintf("策略 '%s' 已停用", s.Name),
			}, nil
		},
	)
}

// DeleteStrategyTool deletes a strategy
func (st *StrategyTools) DeleteStrategyTool() Tool {
	return NewTool(
		"delete_strategy",
		"Delete a strategy permanently.",
		`{"strategy_id": "string (required)"}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			var params struct {
				StrategyID string `json:"strategy_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			if _, ok := st.strategies[params.StrategyID]; !ok {
				return nil, fmt.Errorf("strategy not found: %s", params.StrategyID)
			}

			delete(st.strategies, params.StrategyID)

			return map[string]interface{}{
				"success": true,
				"message": "策略已删除",
			}, nil
		},
	)
}

// GetStrategyTemplates returns available strategy templates
func (st *StrategyTools) GetStrategyTemplates() Tool {
	return NewTool(
		"get_strategy_templates",
		"Get available strategy templates and examples.",
		`{}`,
		func(ctx context.Context, args json.RawMessage) (interface{}, error) {
			templates := []map[string]interface{}{
				{
					"name":        "AI 智能交易",
					"type":        "ai",
					"description": "让 AI 自主分析市场并决策，适合不想手动盯盘的用户",
					"example":     "create_strategy(name='AI智能', description='分析BTC和ETH的技术指标和市场情绪，在有明确趋势时入场')",
				},
				{
					"name":        "网格交易",
					"type":        "grid",
					"description": "在价格区间内自动低买高卖，适合震荡行情",
					"example":     "create_grid_strategy(symbol='BTCUSDT', lower_price=90000, upper_price=100000, grid_count=20, amount_per_grid=100)",
				},
				{
					"name":        "定投 DCA",
					"type":        "dca",
					"description": "定期定额买入，摊薄成本，适合长期投资",
					"example":     "create_dca_strategy(symbol='ETHUSDT', interval_minutes=1440, amount_per_buy=50, max_buys=365)",
				},
				{
					"name":        "趋势跟踪",
					"type":        "trend",
					"description": "跟随趋势，EMA金叉买入死叉卖出",
					"example":     "create_trend_strategy(symbols=['BTCUSDT','ETHUSDT'], ema_fast=9, ema_slow=21, leverage=3)",
				},
				{
					"name":        "RSI 超买超卖",
					"type":        "custom",
					"description": "RSI 低于 30 买入，高于 70 卖出",
					"example":     "create_strategy(name='RSI策略', description='当RSI14低于30时买入，高于70时卖出，止损10%')",
				},
				{
					"name":        "突破策略",
					"type":        "breakout",
					"description": "价格突破关键位时入场",
					"example":     "create_strategy(name='突破策略', description='当价格突破20日最高点时做多，突破20日最低点时做空')",
				},
			}

			return map[string]interface{}{
				"templates": templates,
				"message":   "以上是可用的策略模板，选择一个并告诉我你想怎么定制！",
			}, nil
		},
	)
}
