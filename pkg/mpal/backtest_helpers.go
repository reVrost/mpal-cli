package mpal

import (
	"math"
	"sort"
	"strings"
	"time"
)

func freshnessLabel(freshness *Freshness) string {
	if freshness == nil {
		return "unknown"
	}
	parts := []string{freshness.Source}
	if freshness.Provider != "" {
		parts = append(parts, freshness.Provider)
	}
	if freshness.Storage != "" {
		parts = append(parts, freshness.Storage)
	}
	return strings.Join(parts, "/")
}

func trustedBacktestPriceBlocker(freshness *Freshness) string {
	if freshness == nil {
		return "missing price freshness metadata"
	}
	if freshness.Source == liveYahooHistoricalFetchSource {
		return "live Yahoo historical fetch is not trusted for backtests"
	}
	if freshness.Stale {
		return "price data source is stale"
	}
	storage := strings.ToLower(strings.TrimSpace(freshness.Storage))
	switch storage {
	case trustedHistoricalPriceStorageDynamo, trustedHistoricalPriceStoragePostgres:
		return ""
	default:
		return "price data storage is not trusted for backtests: " + freshnessLabel(freshness)
	}
}

func firstNonEmpty(first string, second string) string {
	if first != "" {
		return first
	}
	return second
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.UTC().Year(), value.UTC().Month(), value.UTC().Day(), 0, 0, 0, 0, time.UTC)
}

func dateString(value time.Time) string {
	return dateOnly(value).Format(time.DateOnly)
}

func backtestCalendar(seriesByTicker map[string][]backtestBar, start time.Time, end time.Time) []time.Time {
	seen := map[string]time.Time{}
	start = dateOnly(start)
	end = dateOnly(end)
	for _, series := range seriesByTicker {
		for _, bar := range series {
			date := dateOnly(bar.Date)
			if date.Before(start) || date.After(end) {
				continue
			}
			seen[dateString(date)] = date
		}
	}
	calendar := make([]time.Time, 0, len(seen))
	for _, date := range seen {
		calendar = append(calendar, date)
	}
	sort.Slice(calendar, func(i, j int) bool { return calendar[i].Before(calendar[j]) })
	return calendar
}

func rebalanceDates(calendar []time.Time, frequency string) []time.Time {
	if len(calendar) == 0 {
		return nil
	}
	frequency = strings.ToLower(strings.TrimSpace(frequency))
	if frequency == "" {
		frequency = "weekly"
	}
	var dates []time.Time
	lastKey := ""
	for _, date := range calendar {
		var key string
		switch frequency {
		case "daily":
			key = dateString(date)
		case "monthly":
			key = date.Format("2006-01")
		default:
			year, week := date.ISOWeek()
			key = strings.Join([]string{itoa(year), itoa(week)}, "-")
		}
		if key == lastKey {
			continue
		}
		lastKey = key
		dates = append(dates, date)
	}
	return dates
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	negative := value < 0
	if negative {
		value = -value
	}
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func dateSet(dates []time.Time) map[string]struct{} {
	out := make(map[string]struct{}, len(dates))
	for _, date := range dates {
		out[dateString(date)] = struct{}{}
	}
	return out
}

func barsThrough(series []backtestBar, asOf time.Time) []backtestBar {
	asOf = dateOnly(asOf)
	idx := sort.Search(len(series), func(i int) bool { return !dateOnly(series[i].Date).Before(asOf.AddDate(0, 0, 1)) })
	return append([]backtestBar(nil), series[:idx]...)
}

func backtestBarsToCoreBars(bars []backtestBar) []Bar {
	out := make([]Bar, 0, len(bars))
	for _, bar := range bars {
		adjustedClose := bar.Close
		out = append(out, Bar{
			Date:          bar.Date,
			Open:          bar.Open,
			High:          bar.High,
			Low:           bar.Low,
			Close:         bar.Close,
			AdjustedClose: &adjustedClose,
		})
	}
	return out
}

func nextAvailableFillDate(seriesByTicker map[string][]backtestBar, date time.Time) (time.Time, bool) {
	var next time.Time
	found := false
	for _, series := range seriesByTicker {
		for _, bar := range series {
			barDate := dateOnly(bar.Date)
			if !barDate.After(dateOnly(date)) {
				continue
			}
			if !found || barDate.Before(next) {
				next = barDate
				found = true
			}
			break
		}
	}
	return next, found
}

func barOnOrBefore(series []backtestBar, date time.Time) (backtestBar, bool) {
	date = dateOnly(date)
	idx := sort.Search(len(series), func(i int) bool { return dateOnly(series[i].Date).After(date) })
	if idx == 0 {
		return backtestBar{}, false
	}
	return series[idx-1], true
}

func barOnDate(series []backtestBar, date time.Time) (backtestBar, bool) {
	date = dateOnly(date)
	idx := sort.Search(len(series), func(i int) bool { return !dateOnly(series[i].Date).Before(date) })
	if idx >= len(series) || !dateOnly(series[idx].Date).Equal(date) {
		return backtestBar{}, false
	}
	return series[idx], true
}

func priceCoverageBlockers(series []backtestBar, start time.Time, end time.Time, toleranceDays int) []string {
	if len(series) == 0 {
		return nil
	}
	tolerance := time.Duration(toleranceDays) * 24 * time.Hour
	var blockers []string
	start = dateOnly(start)
	end = dateOnly(end)
	startBar, ok := barOnOrBefore(series, start)
	if !ok || start.Sub(dateOnly(startBar.Date)) > tolerance {
		blockers = append(blockers, "price coverage does not include requested start "+dateString(start))
	}
	endBar, ok := barOnOrBefore(series, end)
	if !ok || end.Sub(dateOnly(endBar.Date)) > tolerance {
		blockers = append(blockers, "price coverage ends before requested end "+dateString(end))
	}
	return blockers
}

func portfolioFromBacktestPositions(
	seriesByTicker map[string][]backtestBar,
	positions map[string]backtestPosition,
	cash float64,
	date time.Time,
) Portfolio {
	equity, _ := markBacktestEquity(seriesByTicker, positions, cash, date)
	out := Portfolio{Cash: cash, Equity: equity}
	for ticker, position := range nonZeroPositions(positions) {
		bar, ok := barOnOrBefore(seriesByTicker[ticker], date)
		if !ok {
			continue
		}
		value := position.Shares * bar.Close
		weight := 0.0
		if equity > 0 {
			weight = value / equity
		}
		out.Positions = append(out.Positions, Position{
			Ticker:       ticker,
			Shares:       position.Shares,
			MarketValue:  value,
			Weight:       weight,
			CurrentPrice: bar.Close,
		})
	}
	sort.Slice(out.Positions, func(i, j int) bool { return out.Positions[i].Ticker < out.Positions[j].Ticker })
	return out
}

func markBacktestEquity(
	seriesByTicker map[string][]backtestBar,
	positions map[string]backtestPosition,
	cash float64,
	date time.Time,
) (float64, float64) {
	equity := cash
	exposureValue := 0.0
	for ticker, position := range positions {
		if position.Shares <= 0 {
			continue
		}
		bar, ok := barOnOrBefore(seriesByTicker[ticker], date)
		if !ok {
			continue
		}
		value := position.Shares * bar.Close
		equity += value
		exposureValue += value
	}
	exposure := 0.0
	if equity > 0 {
		exposure = exposureValue / equity
	}
	return equity, exposure
}

func nonZeroPositions(positions map[string]backtestPosition) map[string]backtestPosition {
	out := make(map[string]backtestPosition, len(positions))
	for ticker, position := range positions {
		if position.Shares > 0.000001 {
			out[ticker] = position
		}
	}
	return out
}

func cloneBacktestPositions(positions map[string]backtestPosition) map[string]backtestPosition {
	out := make(map[string]backtestPosition, len(positions))
	for ticker, position := range positions {
		out[ticker] = position
	}
	return out
}

func finalBacktestPortfolio(
	seriesByTicker map[string][]backtestBar,
	positions map[string]backtestPosition,
	cash float64,
	date time.Time,
) Portfolio {
	portfolio := portfolioFromBacktestPositions(seriesByTicker, positions, cash, date)
	portfolio.Cash = round(portfolio.Cash, 2)
	portfolio.Equity = round(portfolio.Equity, 2)
	for i := range portfolio.Positions {
		portfolio.Positions[i].Shares = round(portfolio.Positions[i].Shares, 8)
		portfolio.Positions[i].MarketValue = round(portfolio.Positions[i].MarketValue, 2)
		portfolio.Positions[i].Weight = round(portfolio.Positions[i].Weight, 6)
		portfolio.Positions[i].CurrentPrice = round(portfolio.Positions[i].CurrentPrice, 4)
	}
	return portfolio
}

func applyDrawdowns(curve []EquityPoint) {
	peak := 0.0
	for i := range curve {
		if curve[i].Equity > peak {
			peak = curve[i].Equity
		}
		if peak > 0 {
			curve[i].Drawdown = round(curve[i].Equity/peak-1, 6)
		}
	}
}

func dailyReturns(curve []EquityPoint) []float64 {
	if len(curve) < 2 {
		return nil
	}
	out := make([]float64, 0, len(curve)-1)
	for i := 1; i < len(curve); i++ {
		prev := curve[i-1].Equity
		if prev <= 0 {
			continue
		}
		out = append(out, curve[i].Equity/prev-1)
	}
	return out
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	avg := average(values)
	total := 0.0
	for _, value := range values {
		delta := value - avg
		total += delta * delta
	}
	return math.Sqrt(total / float64(len(values)-1))
}

func sortinoRatio(returns []float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	downside := make([]float64, 0, len(returns))
	for _, value := range returns {
		if value < 0 {
			downside = append(downside, value)
		}
	}
	downsideDeviation := stddev(downside) * math.Sqrt(tradingDaysPerYear)
	if downsideDeviation == 0 {
		return 0
	}
	return (average(returns) * tradingDaysPerYear) / downsideDeviation
}

func minDrawdown(curve []EquityPoint) float64 {
	minValue := 0.0
	for _, point := range curve {
		if point.Drawdown < minValue {
			minValue = point.Drawdown
		}
	}
	return minValue
}

func averageCashWeight(curve []EquityPoint) float64 {
	if len(curve) == 0 {
		return 0
	}
	total := 0.0
	for _, point := range curve {
		if point.Equity <= 0 {
			continue
		}
		total += point.Cash / point.Equity
	}
	return total / float64(len(curve))
}
