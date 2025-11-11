package trader

import "math"

// ComputeAdjustedSize computes adjusted position size in USD based on
// recent win rate, profit factor, confidence, recent loss cooldown and margin safety.
// - base: AI-proposed size in USD
// - leverage: position leverage
// - available: available balance in USD
// - feeRate: taker fee rate (e.g., 0.0004)
// - winRate: recent win rate (0-100)
// - profitFactor: recent profit factor (>=0)
// - confidence: decision confidence (0-100)
// - recentLossCooldown: whether to apply cooldown due to recent large loss
func ComputeAdjustedSize(base float64, leverage int, available float64, feeRate float64, winRate float64, profitFactor float64, confidence float64, recentLossCooldown bool) float64 {
    if base <= 0 {
        return base
    }

    // Conservative performance-based multiplier
    multiplier := 1.0
    if winRate >= 60 && profitFactor >= 1.50 {
        multiplier = 1.10
    } else if winRate >= 55 && profitFactor >= 1.30 {
        multiplier = 1.05
    } else if winRate < 45 || profitFactor < 1.00 {
        multiplier = 0.65
    }

    // Confidence gating (more conservative)
    if confidence > 0 {
        if confidence < 50 {
            multiplier *= 0.75
        } else if confidence < 60 {
            multiplier *= 0.85
        }
    }

    // Cooldown on recent large losses
    if recentLossCooldown {
        multiplier *= 0.60
    }

    newSize := base * multiplier

    // Smooth relative-change clamp (easing): limit increases/decreases per decision
    if base > 0 {
        change := newSize / base
        maxIncrease := 1.25 // cap increases to +25%
        maxDecrease := 0.50 // cap decreases to -50%
        if change > maxIncrease {
            newSize = base * maxIncrease
        } else if change < maxDecrease {
            newSize = base * maxDecrease
        }
    }

    // Margin safety clamp: required margin + fee must fit available
    if leverage > 0 && available > 0 {
        denom := (1.0/float64(leverage) + feeRate)
        limit := available / denom
        if limit > 0 {
            // Keep a cushion to reduce near-limit errors
            cushion := 0.92
            effLimit := limit * cushion
            if newSize > effLimit {
                // Logistic compression near the limit for smoother clamping
                ratio := newSize / limit
                compression := 0.92 + (0.08 / (1 + math.Exp(12*(ratio-0.95))))
                newSize = limit * compression
            }
            if newSize > limit {
                newSize = limit
            }
        }
    }

    if newSize < 0 {
        newSize = 0
    }

    return newSize
}