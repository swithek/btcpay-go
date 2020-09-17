package btcpay

import (
	"context"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

// Client
type Client struct {
	hc      *http.Client
	headers map[string]string
}

type setter func(c *Client)

func WithHTTPClient(hc *http.Client) setter {
	return func(c *Client) {
		c.hc = hc
	}
}

func WithUserAgent(ua string) setter {
	return func(c *Client) {
		c.headers["User-Agent"] = ua
	}
}

func NewClient(ss ...setter) *Client {
	c := &Client{
		hc: &http.Client{
			Timeout: time.Second * 20,
		},
	}

	for _, s := range ss {
		s(c)
	}

	return c
}

func (c *Client) send(req *http.Request, sign bool) (*http.Response, error) {
	return nil, nil
}

func (c *Client) GetRates() (map[string]decimal.Decimal, error) {
	return nil, nil
}

type Invoice struct{}

type CreateInvoiceArgs struct{}

func (c *Client) CreateInvoice(ctx context.Context, a CreateInvoiceArgs) (Invoice, error) {
	return Invoice{}, nil
}

func (c *Client) GetInvoice(ctx context.Context, id, token string) (Invoice, error) {
	return Invoice{}, nil
}
