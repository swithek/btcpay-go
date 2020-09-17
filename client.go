package btcpay

import (
	"context"
	"net/http"

	"github.com/shopspring/decimal"
)

// Client
type Client struct {
	http    *http.Client
	headers map[string]string
}

type setter func(c *Client)

func WithHTTPClient(hc *http.Client) setter {
	return func(c *Client) {
		c.http = hc
	}
}

func NewClient(ss ...setter) *Client {
	c := &Client{}

	for _, s := range ss {
		s(c)
	}

	return c
}

func (c *Client) send(req *http.Request) (*http.Response, error) {
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
