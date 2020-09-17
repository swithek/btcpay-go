package btcpay

import (
	"context"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

// Client
type Client struct {
	hc       *http.Client
	headers  map[string]string
	host     string
	pem      string
	clientID string
	token    string
}

type setter func(c *Client)

func HTTPClient(hc *http.Client) setter {
	return func(c *Client) {
		c.hc = hc
	}
}

func UserAgent(ua string) setter {
	return func(c *Client) {
		c.headers["User-Agent"] = ua
	}
}

func NewClient(host, pm, token string, ss ...setter) (*Client, error) {
	c := &Client{
		hc: &http.Client{
			Timeout: time.Second * 20,
		},
		headers: map[string]string{
			"Content-Type":     "application/json",
			"Accept":           "application/json",
			"X-Accept-Version": "2.0.0",
			"User-Agent":       "btcpay-go",
		},
		host:  host,
		pem:   pm,
		token: token,
	}

	for _, s := range ss {
		s(c)
	}

	var err error

	c.clientID, err = generateSIN(pm)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func NewPairedClient(ctx context.Context, host, code string, ss ...setter) (*Client, error) {
	pm, err := GeneratePEM()
	if err != nil {
		return nil, err
	}

	c, err := NewClient(host, pm, "", ss...)
	if err != nil {
		return nil, err
	}

	c.token, err = c.Pair(ctx, code)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) send(req *http.Request, sign bool) (*http.Response, error) {
	return nil, nil
}

func (c *Client) Pair(ctx context.Context, code string) (string, error) {
	return "", nil
}

func (c *Client) GetRates(ctx context.Context) (map[string]decimal.Decimal, error) {
	return nil, nil
}

type Invoice struct{}

type CreateInvoiceArgs struct{}

func (c *Client) CreateInvoice(ctx context.Context, token string, a CreateInvoiceArgs) (Invoice, error) {
	return Invoice{}, nil
}

func (c *Client) GetInvoice(ctx context.Context, token, id string) (Invoice, error) {
	return Invoice{}, nil
}
