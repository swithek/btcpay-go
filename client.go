package btcpay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// Client holds data that is needed to safely communicate with the
// BTCPay server.
type Client struct {
	hc       *http.Client
	headers  map[string]string
	host     string
	pem      string
	clientID string
	pairing  struct {
		sync.RWMutex
		token string
	}
}

type setter func(c *Client)

// WithHTTPClient sets a custom http client on the BTCPay client.
func WithHTTPClient(hc *http.Client) setter { //nolint:golint // setter funcs cannot be created outside of this package
	return func(c *Client) {
		c.hc = hc
	}
}

// WithUserAgent sets a custom user agent string on the BTCPay client.
func WithUserAgent(ua string) setter { //nolint:golint // setter funcs cannot be created outside of this package
	return func(c *Client) {
		c.headers["User-Agent"] = ua
	}
}

// WithPEM sets a custom PEM string on the BTCPay client.
// If not set, it will be generated automatically.
func WithPEM(pm string) setter { //nolint:golint // setter funcs cannot be created outside of this package
	return func(c *Client) {
		c.pem = pm
	}
}

// NewClient creates a fresh instance of BTCPay client.
func NewClient(host, token string, ss ...setter) (*Client, error) {
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
	c.pairing.token = token

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

// NewPairedClient creates a fresh instance of BTCPay client and pairs
// it with the server.
func NewPairedClient(host, code string, ss ...setter) (*Client, error) {
	c, err := NewClient(host, "", ss...)
	if err != nil {
		return nil, err
	}

	if err = c.pair(context.Background(), code); err != nil {
		return nil, err
	}

	return c, nil
}

// send sends an HTTP request to the specified endpoint.
func (c *Client) send(ctx context.Context, method, endpoint string, params url.Values, payload interface{}, sig bool) (*http.Response, error) {
	c.pairing.RLock()
	defer c.pairing.RUnlock()

	var (
		body  *bytes.Buffer
		query strings.Builder // query params order is important
	)

	if payload != nil {
		type pl interface{}

		body = &bytes.Buffer{}
		data := struct {
			pl
			Token string `json:"token,omitempty"`
		}{pl: payload, Token: c.pairing.token}

		if err := json.NewEncoder(body).Encode(data); err != nil {
			return nil, err
		}
	} else {
		if c.pairing.token != "" {
			query.WriteString("token=")
			query.WriteString(c.pairing.token)
		}

		if len(params) > 0 {
			if query.Len() > 0 {
				query.WriteByte('&')
			}

			query.WriteString(params.Encode())
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.host+endpoint, body)
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = query.String()

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	if sig {
		pub, err := pubKey(c.pem)
		if err != nil {
			return nil, err
		}

		req.Header.Set("X-Identity", pub)

		sig, err := sign(c.pem, req.URL.String()+body.String())
		if err != nil {
			return nil, err
		}

		req.Header.Set("X-Signature", sig)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()

		var rerr struct {
			Error string `json:"error"`
		}

		err = json.NewDecoder(resp.Body).Decode(&rerr)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("[%d] %s", resp.StatusCode, rerr.Error)
	}

	return resp, nil
}

// pair pairs the client with the BTCPay server.
func (c *Client) pair(ctx context.Context, code string) error {
	data := struct {
		ID          string `json:"id"`
		PairingCode string `json:"pairingCode"`
	}{
		ID:          c.clientID,
		PairingCode: code,
	}

	resp, err := c.send(ctx, http.MethodPost, "/tokens", nil, data, false)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var tokens []struct {
		Token string `json:"token"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return err
	}

	if len(tokens) == 0 {
		return errors.New("token data not returned")
	}

	c.pairing.Lock()
	c.pairing.token = tokens[0].Token
	c.pairing.Unlock()

	return nil
}

// Rates retrieves exchange rates for each crypto currency paired with
// the provided fiat currency.
// Store ID is optional.
func (c *Client) Rates(ctx context.Context, currency, storeID string) (map[string]decimal.Decimal, error) {
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
	for _, r := range rates.Data {
		rr[r.Code] = r.Rate
	}

	return rr, nil
}

// CreateInvoiceParams holds data used to initialize a new invoice.
// More at: https://bitpay.com/api/#rest-api-resources-invoices-create-an-invoice
type CreateInvoiceParams struct {
	Currency              string          `json:"currency"`
	Price                 decimal.Decimal `json:"price"`
	OrderID               string          `json:"orderId,omitempty"`
	ItemDesc              string          `json:"itemDesc,omitempty"`
	ItemCode              string          `json:"itemCode,omitempty"`
	NotificationEmail     string          `json:"notificationEmail,omitempty"`
	NotificationURL       string          `json:"notificationURL,omitempty"`
	RedirectURL           string          `json:"redirectURL,omitempty"`
	POSData               string          `json:"posData,omitempty"`
	TransactionSpeed      string          `json:"transactionSpeed,omitempty"`
	FullNotifications     bool            `json:"fullNotifications,omitempty"`
	ExtendedNotifications bool            `json:"extendedNotifications,omitempty"`
	Physical              bool            `json:"physical,omitempty"`
	Buyer                 InvoiceBuyer    `json:"buyer"`
	PaymentCurrencies     []string        `json:"paymentCurrencies,omitempty"`
}

// InvoiceBuyer holds buyer information specified during invoice creation.
type InvoiceBuyer struct {
	Name       string `json:"name,omitempty"`
	Address1   string `json:"address1,omitempty"`
	Address2   string `json:"address2,omitempty"`
	Locality   string `json:"locality,omitempty"`
	Region     string `json:"region,omitempty"`
	PostalCode string `json:"postalCode,omitempty"`
	Country    string `json:"country,omitempty"`
	Email      string `json:"email,omitempty"`
	Phone      string `json:"phone,omitempty"`
	Notify     string `json:"notify,omitempty"`
}

// Invoice holds invoice data retrieved from the payment processor.
type Invoice struct {
	URL                 string          `json:"url"`
	POSData             string          `json:"posData"`
	Status              string          `json:"status"`
	Price               decimal.Decimal `json:"price"`
	Currency            string          `json:"currency"`
	ItemDesc            string          `json:"itemDesc"`
	OrderID             string          `json:"orderId"`
	InvoiceTime         int64           `json:"invoiceTime"`
	ExpirationTime      int64           `json:"expirationTime"`
	CurrentTime         int64           `json:"currentTime"`
	ID                  string          `json:"id"`
	LowFeeDetected      bool            `json:"lowFeeDetected"`
	AmountPaid          decimal.Decimal `json:"amountPaid"`
	DisplayAmountPaid   decimal.Decimal `json:"displayAmountPaid"`
	ExceptionStatus     interface{}     `json:"exceptionStatus"`
	TargetConfirmations int64           `json:"targetConfirmations"`
	Buyer               InvoiceBuyer    `json:"buyer"`
	RedirectURL         string          `json:"redirectURL"`
	TransactionCurrency string          `json:"transactionCurrency"`
	UnderpaidAmount     decimal.Decimal `json:"underpaidAmount"`
	OverpaidAmount      decimal.Decimal `json:"overpaidAmount"`
}

// CreateInvoice creates a new invoice by the provided invoice
// creation parameters.
func (c *Client) CreateInvoice(ctx context.Context, p CreateInvoiceParams) (Invoice, error) {
	resp, err := c.send(ctx, http.MethodPost, "/invoices", nil, p, true)
	if err != nil {
		return Invoice{}, err
	}

	defer resp.Body.Close()

	var inv struct {
		Data Invoice `json:"data"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		return Invoice{}, err
	}

	return inv.Data, nil
}

// Invoice retrieves an invoice by the provided ID.
func (c *Client) Invoice(ctx context.Context, id string) (Invoice, error) {
	resp, err := c.send(ctx, http.MethodGet, "/invoices/"+id, nil, nil, true)
	if err != nil {
		return Invoice{}, err
	}

	defer resp.Body.Close()

	var inv struct {
		Data Invoice `json:"data"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		return Invoice{}, err
	}

	return inv.Data, nil
}
