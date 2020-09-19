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
		Resp    httpmock.Responder
		PEM     string
		Sent    bool
		Err     bool
		ErrMsg  string
	}{
		"Error returned during request sending": {
			Resp: httpmock.NewErrorResponder(assert.AnError),
			Sent: true,
			Err:  true,
		},
		"Invalid error response": {
			Resp: httpmock.NewStringResponder(http.StatusUnauthorized, `{"error":"unauthorized123"`),
			Sent: true,
			Err:  true,
		},
		"Error response": {
			Resp:   httpmock.NewStringResponder(http.StatusUnauthorized, `{"error":"unauthorized123"}`),
			Sent:   true,
			Err:    true,
			ErrMsg: "[401] unauthorized123",
		},
		"Successful execution with payload": {
			Payload: CreateInvoiceParams{Currency: "USD"},
			Resp: httpmock.Responder(func(r *http.Request) (*http.Response, error) {
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
			}),
			Sent: true,
			Err:  false,
		},
		"Successful execution with payload and token": {
			Payload: CreateInvoiceParams{Currency: "USD"},
			Token:   "123",
			Resp: httpmock.Responder(func(r *http.Request) (*http.Response, error) {
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
			}),
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
			Resp: httpmock.Responder(func(r *http.Request) (*http.Response, error) {
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
			}),
			Sent: true,
			Err:  false,
		},
		"Successful execution with token": {
			Token: "123",
			Resp: httpmock.Responder(func(r *http.Request) (*http.Response, error) {
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
			}),
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
			Token: "123",
			Resp: httpmock.Responder(func(r *http.Request) (*http.Response, error) {
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
			}),
			Sent: true,
			Err:  false,
		},
		"Successful execution with signature": {
			Payload: CreateInvoiceParams{Currency: "USD"},
			Resp: httpmock.Responder(func(r *http.Request) (*http.Response, error) {
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
			}),
			Sent: true,
			Sig:  true,
			Err:  false,
		},
		"Successful execution": {
			Resp: httpmock.Responder(func(r *http.Request) (*http.Response, error) {
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
			}),
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

			if c.PEM != "" {
				client.pem = c.PEM
			}

			mt.RegisterResponder(http.MethodPost, "http://test.com/testing", c.Resp)

			resp, err := client.send(
				context.Background(),
				http.MethodPost,
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
