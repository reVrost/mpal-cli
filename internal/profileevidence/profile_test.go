package profileevidence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDecodeProfilesKeepsCurrentQuoteFields(t *testing.T) {
	t.Parallel()

	profiles, err := decodeProfiles(`{
		"profiles": {
			"AAPL": {
				"ticker": "AAPL",
				"as_of": "2026-05-10T00:00:00Z",
				"profile_score": 0.7,
				"current_price": 203.45,
				"currency": "USD",
				"quote_time": "2026-05-11T14:30:00Z",
				"market_state": "REGULAR",
				"score_source": "qvm_score"
			}
		}
	}`, []string{"AAPL"})
	require.NoError(t, err)

	profile := profiles["AAPL"]
	require.NotNil(t, profile.CurrentPrice)
	require.Equal(t, 203.45, *profile.CurrentPrice)
	require.Equal(t, "USD", profile.Currency)
	require.Equal(t, "REGULAR", profile.MarketState)
	require.Equal(t, time.Date(2026, 5, 11, 14, 30, 0, 0, time.UTC), *profile.QuoteTime)
}
