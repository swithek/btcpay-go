package btcpay

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
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

func WithPEM(pm string) setter {
	return func(c *Client) {
		c.pem = pm
	}
}

func WithToken(t string) setter {
	return func(c *Client) {
		c.token = t
	}
}

func NewClient(host string, ss ...setter) (*Client, error) {
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
		host: host,
	}

	for _, s := range ss {
		s(c)
	}

	var err error

	if c.pem == "" {
		c.pem, err = generatePEM()
		if err != nil {
			return nil, err
		}
	}

	c.clientID, err = generateSIN(c.pem)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) send(ctx context.Context, method, endpoint string, params url.Values, payload interface{}, sig bool) (*http.Response, error) {
	var (
		body  *bytes.Buffer
		query strings.Builder // query params order is important
	)

	if c.token != "" {
		query.WriteString("token=")
		query.WriteString(c.token)
	}

	if payload != nil {
		type pl interface{}

		body = &bytes.Buffer{}
		data := struct {
			pl
			Token string `json:"token,omitempty"`
		}{pl: payload, Token: c.token}

		if err := json.NewEncoder(body).Encode(data); err != nil {
			return nil, err
		}
	} else if len(params) > 0 {
		if query.Len() > 0 {
			query.WriteByte('&')
		}

		query.WriteString(params.Encode())
	}

	reqURL := c.host + endpoint
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = query.String()

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	if sig {
		pub, err := publicKey(c.pem)
		if err != nil {
			return nil, err
		}

		req.Header.Set("X-Identity", pub)

		sig, err := sign(c.pem, reqURL+body.String())
		if err != nil {
			return nil, err
		}

		req.Header.Set("X-Signature", sig)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}

	// TODO handle error response

	return resp, nil
}

func (c *Client) Pair(ctx context.Context, code string) error {
	data := struct {
		ID          string `json:"id"`
		PairingCode string `json:"pairing_code"`
	}{
		ID:          c.clientID,
		PairingCode: code,
	}

	resp, err := c.send(ctx, http.MethodPost, "/tokens", nil, data, false)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var token struct {
		Token string `json:"token"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return err
	}

	c.token = token.Token

	return nil
}

func (c *Client) GetRates(ctx context.Context, currency, storeID string) (map[string]decimal.Decimal, error) {
	var params url.Values
	params.Set("cryptoCode", currency)

	if storeID != "" {
		params.Set("storeID", storeID)
	}

	resp, err := c.send(ctx, http.MethodGet, "/rates", params, nil, true)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var rates struct {
		Data []struct {
			Code string          `json:"code"`
			Rate decimal.Decimal `json:"rate"`
		} `json:"data"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		return nil, err
	}

	rr := make(map[string]decimal.Decimal, len(rates.Data))
	for c, r := range rates.Data {
		rr[c] = r
	}

	return rr, nil
}

type Invoice struct{}

type CreateInvoiceArgs struct{}

func (c *Client) CreateInvoice(ctx context.Context, token string, a CreateInvoiceArgs) (Invoice, error) {
	return Invoice{}, nil
}

func (c *Client) GetInvoice(ctx context.Context, token, id string) (Invoice, error) {
	return Invoice{}, nil
}
