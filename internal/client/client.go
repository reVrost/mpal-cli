package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"connectrpc.com/connect"
	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	"github.com/revrost/mpal-cli/gen/marketpal/v1/marketpalv1connect"
)

const defaultMpalBaseURL = "https://api.marketpal.ai"

type Client struct {
	apiKey string
	client marketpalv1connect.MpalServiceClient
}

type API interface {
	GetTickerEvents(ctx context.Context, msg *marketpalv1.MpalTickerEventsRequest) (string, error)
	GetTickerBars(ctx context.Context, msg *marketpalv1.MpalTickerBarsRequest) (string, error)
	GetTickerProfile(ctx context.Context, msg *marketpalv1.MpalTickerProfileRequest) (string, error)
	GetTickerFinancials(ctx context.Context, msg *marketpalv1.MpalTickerFinancialsRequest) (string, error)
	GetTickerFundamentals(ctx context.Context, msg *marketpalv1.MpalTickerDataRequest) (string, error)
	GetTickerInsiders(ctx context.Context, msg *marketpalv1.MpalTickerDataRequest) (string, error)
	GetTickerOwnership(ctx context.Context, msg *marketpalv1.MpalTickerDataRequest) (string, error)
	GetPortfolioSnapshot(ctx context.Context, msg *marketpalv1.MpalPortfolioSnapshotRequest) (string, error)
	GetWatchlist(ctx context.Context, msg *marketpalv1.MpalWatchlistRequest) (string, error)
	RunStrategy(ctx context.Context, msg *marketpalv1.MpalStrategyRunRequest) (string, error)
	RunBacktest(ctx context.Context, msg *marketpalv1.MpalBacktestRunRequest) (string, error)
}

func NewFromEnv() *Client {
	baseURL := firstNonEmpty(
		strings.TrimRight(strings.TrimSpace(getenv("MPAL_BASE_URL")), "/"),
		defaultMpalBaseURL,
	)
	return &Client{
		apiKey: firstNonEmpty(getenv("MPAL_API_KEY"), getenv("MPAL_API_KEYS")),
		client: marketpalv1connect.NewMpalServiceClient(
			http.DefaultClient,
			baseURL,
		),
	}
}

func (c *Client) GetTickerEvents(ctx context.Context, msg *marketpalv1.MpalTickerEventsRequest) (string, error) {
	resp, err := c.client.GetTickerEvents(ctx, authRequest(c.apiKey, msg))
	return payload(resp, err)
}

func (c *Client) GetTickerBars(ctx context.Context, msg *marketpalv1.MpalTickerBarsRequest) (string, error) {
	resp, err := c.client.GetTickerBars(ctx, authRequest(c.apiKey, msg))
	return payload(resp, err)
}

func (c *Client) GetTickerProfile(ctx context.Context, msg *marketpalv1.MpalTickerProfileRequest) (string, error) {
	resp, err := c.client.GetTickerProfile(ctx, authRequest(c.apiKey, msg))
	return payload(resp, err)
}

func (c *Client) GetTickerFinancials(ctx context.Context, msg *marketpalv1.MpalTickerFinancialsRequest) (string, error) {
	resp, err := c.client.GetTickerFinancials(ctx, authRequest(c.apiKey, msg))
	return payload(resp, err)
}

func (c *Client) GetTickerFundamentals(ctx context.Context, msg *marketpalv1.MpalTickerDataRequest) (string, error) {
	resp, err := c.client.GetTickerFundamentals(ctx, authRequest(c.apiKey, msg))
	return payload(resp, err)
}

func (c *Client) GetTickerInsiders(ctx context.Context, msg *marketpalv1.MpalTickerDataRequest) (string, error) {
	resp, err := c.client.GetTickerInsiders(ctx, authRequest(c.apiKey, msg))
	return payload(resp, err)
}

func (c *Client) GetTickerOwnership(ctx context.Context, msg *marketpalv1.MpalTickerDataRequest) (string, error) {
	resp, err := c.client.GetTickerOwnership(ctx, authRequest(c.apiKey, msg))
	return payload(resp, err)
}

func (c *Client) GetPortfolioSnapshot(ctx context.Context, msg *marketpalv1.MpalPortfolioSnapshotRequest) (string, error) {
	resp, err := c.client.GetPortfolioSnapshot(ctx, authRequest(c.apiKey, msg))
	return payload(resp, err)
}

func (c *Client) GetWatchlist(ctx context.Context, msg *marketpalv1.MpalWatchlistRequest) (string, error) {
	resp, err := c.client.GetWatchlist(ctx, authRequest(c.apiKey, msg))
	return payload(resp, err)
}

func (c *Client) RunStrategy(ctx context.Context, msg *marketpalv1.MpalStrategyRunRequest) (string, error) {
	resp, err := c.client.RunStrategy(ctx, authRequest(c.apiKey, msg))
	return payload(resp, err)
}

func (c *Client) RunBacktest(ctx context.Context, msg *marketpalv1.MpalBacktestRunRequest) (string, error) {
	resp, err := c.client.RunBacktest(ctx, authRequest(c.apiKey, msg))
	return payload(resp, err)
}

func authRequest[T any](apiKey string, msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	if strings.TrimSpace(apiKey) != "" {
		req.Header().Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}
	return req
}

func payload(resp *connect.Response[marketpalv1.MpalJSONResponse], err error) (string, error) {
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Msg == nil {
		return "", fmt.Errorf("empty mpal api response")
	}
	return resp.Msg.PayloadJson, nil
}

func getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
