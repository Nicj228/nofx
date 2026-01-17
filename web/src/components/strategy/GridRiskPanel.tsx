import { useState, useEffect, useCallback } from 'react'
import { Shield, TrendingUp, AlertTriangle, Activity, Box } from 'lucide-react'
import type { GridRiskInfo } from '../../types'

interface GridRiskPanelProps {
  traderId: string
  language?: string
  refreshInterval?: number // ms, default 5000
}

export function GridRiskPanel({
  traderId,
  language = 'en',
  refreshInterval = 5000,
}: GridRiskPanelProps) {
  const [riskInfo, setRiskInfo] = useState<GridRiskInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      // Section titles
      leverageInfo: { zh: '杠杆信息', en: 'Leverage Info' },
      positionInfo: { zh: '仓位信息', en: 'Position Info' },
      liquidationInfo: { zh: '清算信息', en: 'Liquidation Info' },
      marketState: { zh: '市场状态', en: 'Market State' },
      boxState: { zh: '盒子状态', en: 'Box State' },

      // Leverage
      currentLeverage: { zh: '当前杠杆', en: 'Current Leverage' },
      effectiveLeverage: { zh: '有效杠杆', en: 'Effective Leverage' },
      recommendedLeverage: { zh: '建议杠杆', en: 'Recommended Leverage' },

      // Position
      currentPosition: { zh: '当前仓位', en: 'Current Position' },
      maxPosition: { zh: '最大仓位', en: 'Max Position' },
      positionPercent: { zh: '仓位占比', en: 'Position %' },

      // Liquidation
      liquidationPrice: { zh: '清算价格', en: 'Liquidation Price' },
      liquidationDistance: { zh: '清算距离', en: 'Liquidation Distance' },

      // Market
      regimeLevel: { zh: '波动级别', en: 'Regime Level' },
      currentPrice: { zh: '当前价格', en: 'Current Price' },
      breakoutLevel: { zh: '突破级别', en: 'Breakout Level' },
      breakoutDirection: { zh: '突破方向', en: 'Breakout Direction' },

      // Box
      shortBox: { zh: '短期盒子', en: 'Short Box' },
      midBox: { zh: '中期盒子', en: 'Mid Box' },
      longBox: { zh: '长期盒子', en: 'Long Box' },

      // Regime levels
      narrow: { zh: '窄幅震荡', en: 'Narrow' },
      standard: { zh: '标准震荡', en: 'Standard' },
      wide: { zh: '宽幅震荡', en: 'Wide' },
      volatile: { zh: '剧烈震荡', en: 'Volatile' },
      trending: { zh: '趋势', en: 'Trending' },

      // Breakout levels
      none: { zh: '无', en: 'None' },
      short: { zh: '短期', en: 'Short' },
      mid: { zh: '中期', en: 'Mid' },
      long: { zh: '长期', en: 'Long' },

      // Directions
      up: { zh: '向上', en: 'Up' },
      down: { zh: '向下', en: 'Down' },

      // Status
      loading: { zh: '加载中...', en: 'Loading...' },
      error: { zh: '加载失败', en: 'Load Failed' },
      noData: { zh: '暂无数据', en: 'No Data' },
    }
    return translations[key]?.[language] || key
  }

  const fetchRiskInfo = useCallback(async () => {
    try {
      const token = localStorage.getItem('token')
      const response = await fetch(`/api/traders/${traderId}/grid-risk`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      })

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }

      const data = await response.json()
      setRiskInfo(data)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }, [traderId])

  useEffect(() => {
    fetchRiskInfo()
    const interval = setInterval(fetchRiskInfo, refreshInterval)
    return () => clearInterval(interval)
  }, [fetchRiskInfo, refreshInterval])

  const getRegimeColor = (regime: string) => {
    switch (regime) {
      case 'narrow':
        return '#0ECB81' // Green - safe
      case 'standard':
        return '#F0B90B' // Yellow - normal
      case 'wide':
        return '#F7931A' // Orange - caution
      case 'volatile':
        return '#F6465D' // Red - danger
      case 'trending':
        return '#8B5CF6' // Purple - trending
      default:
        return '#848E9C' // Gray
    }
  }

  const getBreakoutColor = (level: string) => {
    switch (level) {
      case 'none':
        return '#0ECB81' // Green - safe
      case 'short':
        return '#F0B90B' // Yellow - minor
      case 'mid':
        return '#F7931A' // Orange - warning
      case 'long':
        return '#F6465D' // Red - critical
      default:
        return '#848E9C'
    }
  }

  const getPositionColor = (percent: number) => {
    if (percent < 50) return '#0ECB81' // Green
    if (percent < 80) return '#F0B90B' // Yellow
    return '#F6465D' // Red
  }

  const formatPrice = (price: number) => {
    if (price === 0) return '-'
    if (price >= 1000) return price.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
    if (price >= 1) return price.toFixed(4)
    return price.toFixed(6)
  }

  const formatUSD = (value: number) => {
    return `$${value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
  }

  const sectionStyle = {
    background: '#0B0E11',
    border: '1px solid #2B3139',
  }

  const labelStyle = { color: '#848E9C' }
  const valueStyle = { color: '#EAECEF' }

  if (loading) {
    return (
      <div className="p-4 text-center" style={{ color: '#848E9C' }}>
        {t('loading')}
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-4 text-center" style={{ color: '#F6465D' }}>
        {t('error')}: {error}
      </div>
    )
  }

  if (!riskInfo) {
    return (
      <div className="p-4 text-center" style={{ color: '#848E9C' }}>
        {t('noData')}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Leverage Info */}
      <div className="p-4 rounded-lg" style={sectionStyle}>
        <div className="flex items-center gap-2 mb-3">
          <TrendingUp className="w-4 h-4" style={{ color: '#F0B90B' }} />
          <span className="font-medium text-sm" style={{ color: '#EAECEF' }}>
            {t('leverageInfo')}
          </span>
        </div>
        <div className="grid grid-cols-3 gap-4">
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('currentLeverage')}
            </div>
            <div className="text-lg font-mono" style={valueStyle}>
              {riskInfo.current_leverage}x
            </div>
          </div>
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('effectiveLeverage')}
            </div>
            <div className="text-lg font-mono" style={{ color: '#F0B90B' }}>
              {riskInfo.effective_leverage.toFixed(2)}x
            </div>
          </div>
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('recommendedLeverage')}
            </div>
            <div
              className="text-lg font-mono"
              style={{
                color:
                  riskInfo.current_leverage > riskInfo.recommended_leverage
                    ? '#F6465D'
                    : '#0ECB81',
              }}
            >
              {riskInfo.recommended_leverage}x
            </div>
          </div>
        </div>
      </div>

      {/* Position Info */}
      <div className="p-4 rounded-lg" style={sectionStyle}>
        <div className="flex items-center gap-2 mb-3">
          <Activity className="w-4 h-4" style={{ color: '#F0B90B' }} />
          <span className="font-medium text-sm" style={{ color: '#EAECEF' }}>
            {t('positionInfo')}
          </span>
        </div>
        <div className="grid grid-cols-3 gap-4 mb-3">
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('currentPosition')}
            </div>
            <div className="text-lg font-mono" style={valueStyle}>
              {formatUSD(riskInfo.current_position)}
            </div>
          </div>
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('maxPosition')}
            </div>
            <div className="text-lg font-mono" style={valueStyle}>
              {formatUSD(riskInfo.max_position)}
            </div>
          </div>
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('positionPercent')}
            </div>
            <div
              className="text-lg font-mono"
              style={{ color: getPositionColor(riskInfo.position_percent) }}
            >
              {riskInfo.position_percent.toFixed(1)}%
            </div>
          </div>
        </div>
        {/* Position Progress Bar */}
        <div className="h-2 rounded-full overflow-hidden" style={{ background: '#1E2329' }}>
          <div
            className="h-full rounded-full transition-all duration-300"
            style={{
              width: `${Math.min(riskInfo.position_percent, 100)}%`,
              background: getPositionColor(riskInfo.position_percent),
            }}
          />
        </div>
      </div>

      {/* Liquidation Info */}
      <div className="p-4 rounded-lg" style={sectionStyle}>
        <div className="flex items-center gap-2 mb-3">
          <AlertTriangle className="w-4 h-4" style={{ color: '#F6465D' }} />
          <span className="font-medium text-sm" style={{ color: '#EAECEF' }}>
            {t('liquidationInfo')}
          </span>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('liquidationPrice')}
            </div>
            <div className="text-lg font-mono" style={{ color: '#F6465D' }}>
              {riskInfo.liquidation_price > 0 ? formatPrice(riskInfo.liquidation_price) : '-'}
            </div>
          </div>
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('liquidationDistance')}
            </div>
            <div className="text-lg font-mono" style={{ color: '#F6465D' }}>
              {riskInfo.liquidation_distance.toFixed(1)}%
            </div>
          </div>
        </div>
      </div>

      {/* Market State */}
      <div className="p-4 rounded-lg" style={sectionStyle}>
        <div className="flex items-center gap-2 mb-3">
          <Shield className="w-4 h-4" style={{ color: '#F0B90B' }} />
          <span className="font-medium text-sm" style={{ color: '#EAECEF' }}>
            {t('marketState')}
          </span>
        </div>
        <div className="grid grid-cols-2 gap-4 mb-3">
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('regimeLevel')}
            </div>
            <div
              className="text-lg font-medium"
              style={{ color: getRegimeColor(riskInfo.regime_level) }}
            >
              {t(riskInfo.regime_level || 'standard')}
            </div>
          </div>
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('currentPrice')}
            </div>
            <div className="text-lg font-mono" style={valueStyle}>
              {formatPrice(riskInfo.current_price)}
            </div>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('breakoutLevel')}
            </div>
            <div
              className="text-lg font-medium"
              style={{ color: getBreakoutColor(riskInfo.breakout_level) }}
            >
              {t(riskInfo.breakout_level || 'none')}
            </div>
          </div>
          <div>
            <div className="text-xs" style={labelStyle}>
              {t('breakoutDirection')}
            </div>
            <div
              className="text-lg font-medium"
              style={{
                color: riskInfo.breakout_direction === 'up' ? '#0ECB81' : riskInfo.breakout_direction === 'down' ? '#F6465D' : '#848E9C',
              }}
            >
              {riskInfo.breakout_direction ? t(riskInfo.breakout_direction) : '-'}
            </div>
          </div>
        </div>
      </div>

      {/* Box State */}
      <div className="p-4 rounded-lg" style={sectionStyle}>
        <div className="flex items-center gap-2 mb-3">
          <Box className="w-4 h-4" style={{ color: '#F0B90B' }} />
          <span className="font-medium text-sm" style={{ color: '#EAECEF' }}>
            {t('boxState')}
          </span>
        </div>
        <div className="space-y-3">
          {/* Short Box */}
          <div className="flex items-center justify-between">
            <span className="text-xs" style={labelStyle}>
              {t('shortBox')}
            </span>
            <span className="text-sm font-mono" style={valueStyle}>
              {formatPrice(riskInfo.short_box_lower)} - {formatPrice(riskInfo.short_box_upper)}
            </span>
          </div>
          {/* Mid Box */}
          <div className="flex items-center justify-between">
            <span className="text-xs" style={labelStyle}>
              {t('midBox')}
            </span>
            <span className="text-sm font-mono" style={valueStyle}>
              {formatPrice(riskInfo.mid_box_lower)} - {formatPrice(riskInfo.mid_box_upper)}
            </span>
          </div>
          {/* Long Box */}
          <div className="flex items-center justify-between">
            <span className="text-xs" style={labelStyle}>
              {t('longBox')}
            </span>
            <span className="text-sm font-mono" style={valueStyle}>
              {formatPrice(riskInfo.long_box_lower)} - {formatPrice(riskInfo.long_box_upper)}
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}
