package btcpay

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_WithHTTPClient(t *testing.T) {
	c := &Client{}
	WithHTTPClient(&http.Client{})(c)
	assert.NotNil(t, c.hc)
}

func Test_WithUserAgent(t *testing.T) {
	c := &Client{header: make(map[string]string)}
	WithUserAgent("test")(c)
	assert.Equal(t, "test", c.header["User-Agent"])
}

func Test_WithPEM(t *testing.T) {
	c := &Client{}
	WithPEM("test")(c)
	assert.Equal(t, "test", c.pem)
}

func Test_NewClient(t *testing.T) {
	c, err := NewClient("test123", "test222")
	assert.NoError(t, err)
	require.NotNil(t, c)
	assert.NotNil(t, c.hc)
	assert.Len(t, c.header, 4)
	assert.Equal(t, "test123", c.host)
	assert.Equal(t, "test222", c.token)
	assert.NotZero(t, c.pem)
	assert.NotZero(t, c.clientID)
}

func Test_NewPairedClient(t *testing.T) {
	mt := httpmock.NewMockTransport()
	mt.RegisterResponder(http.MethodPost, "http://test.com/tokens", httpmock.NewErrorResponder(assert.AnError))

	c, err := NewPairedClient("http://test.com", "test222", WithHTTPClient(&http.Client{Transport: mt}))
	assert.Error(t, err)
	assert.Nil(t, c)

	// success
	mt = httpmock.NewMockTransport()
	mt.RegisterResponder(http.MethodPost, "http://test.com/tokens", httpmock.NewStringResponder(http.StatusOK, `[{"token":"123"}]`))

	c, err = NewPairedClient("http://test.com", "test222", WithHTTPClient(&http.Client{Transport: mt}))
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, "123", c.token)
}

func Test_Client_Token(t *testing.T) {
	c := &Client{token: "123"}
	assert.Equal(t, "123", c.Token())
}

func Test_Client_send(t *testing.T) {
	checkHeader := func(h http.Header, sig bool) error {
		if h.Get("Content-Type") != "application/json" ||
			h.Get("Accept") != "application/json" ||
			h.Get("X-Accept-Version") != "2.0.0" ||
			h.Get("User-Agent") != "btcpay-go" {
			return errors.New("invalid header")
		}

		if sig && (h.Get("X-Identity") == "" || h.Get("X-Signature") == "") {
			return errors.New("invalid sig header")
		}

		return nil
	}

	cc := map[string]struct {
		Params  url.Values
		Payload interface{}
		Sig     bool
		Token   string
		Method  string
		Resp    httpmock.Responder
		Sent    bool
		Err     bool
		ErrMsg  string
	}{
		"Invalid payload": {
			Payload: func() {},
			Method:  http.MethodPost,
			Resp:    httpmock.NewStringResponder(http.StatusOK, ""),
			Err:     true,
		},
		"Error returned during payload unmarshal": {
			Payload: 123,
			Token:   "123",
			Method:  http.MethodPost,
			Resp:    httpmock.NewStringResponder(http.StatusOK, ""),
			Err:     true,
		},
		"Invalid method": {
			Method: "[[123",
			Resp:   httpmock.NewStringResponder(http.StatusOK, ""),
			Err:    true,
		},
		"Error returned during request sending": {
			Method: http.MethodPost,
			Resp:   httpmock.NewErrorResponder(assert.AnError),
			Sent:   true,
			Err:    true,
		},
		"Invalid error response": {
			Method: http.MethodPost,
			Resp:   httpmock.NewStringResponder(http.StatusUnauthorized, `{"error":"unauthorized123"`),
			Sent:   true,
			Err:    true,
		},
		"Error response": {
			Method: http.MethodPost,
			Resp:   httpmock.NewStringResponder(http.StatusUnauthorized, `{"error":"unauthorized123"}`),
			Sent:   true,
			Err:    true,
			ErrMsg: "[401] unauthorized123",
		},
		"Successful execution with payload": {
			Payload: CreateInvoiceParams{Currency: "USD"},
			Method:  http.MethodPost,
			Resp: func(r *http.Request) (*http.Response, error) {
				if len(r.URL.Query()) > 0 {
					return nil, errors.New("invalid query params")
				}

				if err := checkHeader(r.Header, false); err != nil {
					return nil, err
				}

				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}

				pl, err := json.Marshal(CreateInvoiceParams{Currency: "USD"})
				if err != nil {
					return nil, errors.New("invalid payload")
				}

				if string(b) != string(pl) {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponse(http.StatusOK, ""), nil
			},
			Sent: true,
			Err:  false,
		},
		"Successful execution with payload and token": {
			Payload: CreateInvoiceParams{Currency: "USD"},
			Token:   "123",
			Method:  http.MethodPost,
			Resp: func(r *http.Request) (*http.Response, error) {
				if len(r.URL.Query()) > 0 {
					return nil, errors.New("invalid query params")
				}

				if err := checkHeader(r.Header, false); err != nil {
					return nil, err
				}

				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}

				pl, err := json.Marshal(CreateInvoiceParams{Currency: "USD"})
				if err != nil {
					panic(err)
				}

				m := make(map[string]interface{})
				if err = json.Unmarshal(pl, &m); err != nil {
					panic(err)
				}

				m["token"] = "123"

				pl, err = json.Marshal(m)
				if err != nil {
					panic(err)
				}

				if string(b) != string(pl) {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponse(http.StatusOK, ""), nil
			},
			Sent: true,
			Err:  false,
		},
		"Successful execution with query params": {
			Params: func() url.Values {
				p := url.Values{}
				p.Set("q1", "v1")
				p.Set("q2", "v2")
				return p
			}(),
			Method: http.MethodPost,
			Resp: func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Get("q1") == "" ||
					r.URL.Query().Get("q2") == "" {
					return nil, errors.New("invalid query params")
				}

				if err := checkHeader(r.Header, false); err != nil {
					return nil, err
				}

				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}

				if len(b) > 0 {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponse(http.StatusOK, ""), nil
			},
			Sent: true,
			Err:  false,
		},
		"Successful execution with token": {
			Token:  "123",
			Method: http.MethodPost,
			Resp: func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Get("token") != "123" {
					return nil, errors.New("invalid query params")
				}

				if err := checkHeader(r.Header, false); err != nil {
					return nil, err
				}

				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}

				if len(b) > 0 {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponse(http.StatusOK, ""), nil
			},
			Sent: true,
			Err:  false,
		},
		"Successful execution with query params and token": {
			Params: func() url.Values {
				p := url.Values{}
				p.Set("q1", "v1")
				p.Set("q2", "v2")
				return p
			}(),
			Token:  "123",
			Method: http.MethodPost,
			Resp: func(r *http.Request) (*http.Response, error) {
				if r.URL.RawQuery != "token=123&q1=v1&q2=v2" {
					return nil, errors.New("invalid query params")
				}

				if err := checkHeader(r.Header, false); err != nil {
					return nil, err
				}

				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}

				if len(b) > 0 {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponse(http.StatusOK, ""), nil
			},
			Sent: true,
			Err:  false,
		},
		"Successful execution with signature": {
			Payload: CreateInvoiceParams{Currency: "USD"},
			Method:  http.MethodPost,
			Resp: func(r *http.Request) (*http.Response, error) {
				if len(r.URL.Query()) > 0 {
					return nil, errors.New("invalid query params")
				}

				if err := checkHeader(r.Header, true); err != nil {
					return nil, err
				}

				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}

				pl, err := json.Marshal(CreateInvoiceParams{Currency: "USD"})
				if err != nil {
					return nil, errors.New("invalid payload")
				}

				if string(b) != string(pl) {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponse(http.StatusOK, ""), nil
			},
			Sent: true,
			Sig:  true,
			Err:  false,
		},
		"Successful execution": {
			Method: http.MethodPost,
			Resp: func(r *http.Request) (*http.Response, error) {
				if len(r.URL.Query()) > 0 {
					return nil, errors.New("invalid query params")
				}

				if err := checkHeader(r.Header, false); err != nil {
					return nil, err
				}

				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}

				if len(b) > 0 {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponse(http.StatusOK, ""), nil
			},
			Sent: true,
			Err:  false,
		},
	}

	for cn, c := range cc {
		c := c

		t.Run(cn, func(t *testing.T) {
			t.Parallel()

			mt := httpmock.NewMockTransport()
			client, err := NewClient("http://test.com", c.Token, WithHTTPClient(&http.Client{Transport: mt}))
			require.NoError(t, err)

			mt.RegisterResponder(http.MethodPost, "http://test.com/testing", c.Resp)

			resp, err := client.send(
				context.Background(),
				c.Method,
				"/testing",
				c.Params,
				c.Payload,
				c.Sig,
			)

			if c.Sent {
				assert.Equal(t, 1, mt.GetCallCountInfo()[http.MethodPost+" http://test.com/testing"])
			}

			if c.Err {
				assert.Error(t, err)
				assert.Nil(t, resp)

				if c.ErrMsg != "" {
					assert.EqualError(t, err, c.ErrMsg)
				}

				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}

func Test_Client_pair(t *testing.T) {
	cc := map[string]struct {
		Code   string
		Resp   httpmock.Responder
		Err    bool
		ErrMsg string
		Result string
	}{
		"Error returned during request sending": {
			Code: "12345",
			Resp: httpmock.NewErrorResponder(assert.AnError),
			Err:  true,
		},
		"Invalid response body": {
			Code: "12345",
			Resp: func(r *http.Request) (*http.Response, error) {
				var data struct {
					ID          string `json:"id"`
					PairingCode string `json:"pairingCode"`
				}

				err := json.NewDecoder(r.Body).Decode(&data)
				if err != nil {
					return nil, err
				}

				if data.ID == "" || data.PairingCode != "12345" {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponder(http.StatusOK, `[`)(r)
			},
			Err: true,
		},
		"No tokens returned": {
			Code: "12345",
			Resp: func(r *http.Request) (*http.Response, error) {
				var data struct {
					ID          string `json:"id"`
					PairingCode string `json:"pairingCode"`
				}

				err := json.NewDecoder(r.Body).Decode(&data)
				if err != nil {
					return nil, err
				}

				if data.ID == "" || data.PairingCode != "12345" {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponder(http.StatusOK, `[]`)(r)
			},
			Err:    true,
			ErrMsg: "token data not returned",
		},
		"Successful execution": {
			Code: "12345",
			Resp: func(r *http.Request) (*http.Response, error) {
				var data struct {
					ID          string `json:"id"`
					PairingCode string `json:"pairingCode"`
				}

				err := json.NewDecoder(r.Body).Decode(&data)
				if err != nil {
					return nil, err
				}

				if data.ID == "" || data.PairingCode != "12345" {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponder(http.StatusOK, `[{"token":"tok123"}]`)(r)
			},
			Result: "tok123",
		},
	}

	for cn, c := range cc {
		c := c

		t.Run(cn, func(t *testing.T) {
			t.Parallel()

			mt := httpmock.NewMockTransport()
			client, err := NewClient("http://test.com", "", WithHTTPClient(&http.Client{Transport: mt}))
			require.NoError(t, err)

			mt.RegisterResponder(http.MethodPost, "http://test.com/tokens", c.Resp)

			err = client.pair(context.Background(), c.Code)

			assert.Equal(t, 1, mt.GetCallCountInfo()[http.MethodPost+" http://test.com/tokens"])

			if c.Err {
				assert.Error(t, err)
				assert.Zero(t, client.token)

				if c.ErrMsg != "" {
					assert.EqualError(t, err, c.ErrMsg)
				}

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, c.Result, client.token)
		})
	}
}

func Test_Client_CreateInvoice(t *testing.T) {
	cc := map[string]struct {
		Params CreateInvoiceParams
		Resp   httpmock.Responder
		Result Invoice
		Err    bool
	}{
		"Error returned during request sending": {
			Params: CreateInvoiceParams{
				Currency: "USD",
			},
			Resp: func(r *http.Request) (*http.Response, error) {
				d, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}

				inv := CreateInvoiceParams{
					Currency: "USD",
				}

				d1, err := json.Marshal(inv)
				if err != nil {
					return nil, err
				}

				if string(d) != string(d1) {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewErrorResponder(assert.AnError)(r)
			},
			Err: true,
		},
		"Invalid response body": {
			Params: CreateInvoiceParams{
				Currency: "USD",
			},
			Resp: func(r *http.Request) (*http.Response, error) {
				d, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}

				inv := CreateInvoiceParams{
					Currency: "USD",
				}

				d1, err := json.Marshal(inv)
				if err != nil {
					return nil, err
				}

				if string(d) != string(d1) {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponder(http.StatusOK, `{`)(r)
			},
			Err: true,
		},
		"Successful execution": {
			Params: CreateInvoiceParams{
				Currency: "USD",
			},
			Resp: func(r *http.Request) (*http.Response, error) {
				d, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}

				inv := CreateInvoiceParams{
					Currency: "USD",
				}

				d1, err := json.Marshal(inv)
				if err != nil {
					return nil, err
				}

				if string(d) != string(d1) {
					return nil, errors.New("invalid body")
				}

				return httpmock.NewStringResponder(http.StatusOK, `{"data":{"id":"12345"}}`)(r)
			},
			Result: Invoice{ID: "12345"},
		},
	}

	for cn, c := range cc {
		c := c

		t.Run(cn, func(t *testing.T) {
			t.Parallel()

			mt := httpmock.NewMockTransport()
			client, err := NewClient("http://test.com", "", WithHTTPClient(&http.Client{Transport: mt}))
			require.NoError(t, err)

			mt.RegisterResponder(http.MethodPost, "http://test.com/invoices", c.Resp)

			inv, err := client.CreateInvoice(context.Background(), c.Params)

			assert.Equal(t, 1, mt.GetCallCountInfo()[http.MethodPost+" http://test.com/invoices"])

			if c.Err {
				assert.Error(t, err)
				assert.Zero(t, inv)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, c.Result, inv)
		})
	}
}

func Test_Client_Invoice(t *testing.T) {
	cc := map[string]struct {
		ID     string
		Resp   httpmock.Responder
		Result Invoice
		Err    bool
	}{
		"Error returned during request sending": {
			ID:   "123",
			Resp: httpmock.NewErrorResponder(assert.AnError),
			Err:  true,
		},
		"Invalid response body": {
			ID:   "123",
			Resp: httpmock.NewStringResponder(http.StatusOK, "{"),
			Err:  true,
		},
		"Successful execution": {
			ID:     "123",
			Resp:   httpmock.NewStringResponder(http.StatusOK, `{"data":{"currency":"USD"}}`),
			Result: Invoice{Currency: "USD"},
		},
	}

	for cn, c := range cc {
		c := c

		t.Run(cn, func(t *testing.T) {
			t.Parallel()

			mt := httpmock.NewMockTransport()
			client, err := NewClient("http://test.com", "", WithHTTPClient(&http.Client{Transport: mt}))
			require.NoError(t, err)

			mt.RegisterResponder(http.MethodGet, "http://test.com/invoices/"+c.ID, c.Resp)

			inv, err := client.Invoice(context.Background(), c.ID)

			assert.Equal(t, 1, mt.GetCallCountInfo()[http.MethodGet+" http://test.com/invoices/"+c.ID])

			if c.Err {
				assert.Error(t, err)
				assert.Zero(t, inv)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, c.Result, inv)
		})
	}
}
