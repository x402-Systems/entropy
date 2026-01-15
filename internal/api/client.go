package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/x402-Systems/entropy/internal/config"

	x402 "github.com/coinbase/x402/go"
	x402http "github.com/coinbase/x402/go/http"
	evm "github.com/coinbase/x402/go/mechanisms/evm/exact/client"
	evmsigners "github.com/coinbase/x402/go/signers/evm"
	"github.com/zalando/go-keyring"
)

type Client struct {
	HTTPClient *http.Client
	PayerID    string
}

type RemoteVM struct {
	ProviderID    int64     `json:"ProviderID"`
	Status        string    `json:"Status"`
	IP            string    `json:"IP"`
	ExpiresAt     time.Time `json:"ExpiresAt"`
	TimeRemaining string    `json:"time_remaining"`
}

type ListResponse struct {
	Count int        `json:"count"`
	VMs   []RemoteVM `json:"vms"`
}

// NewClient initializes the x402 payment-wrapped HTTP client
func NewClient(preference string) (*Client, error) {
	selector := func(reqs []x402.PaymentRequirementsView) x402.PaymentRequirementsView {
		for _, r := range reqs {
			net := strings.ToLower(string(r.GetNetwork()))
			if preference == "xmr" && strings.Contains(net, "monero") {
				return r
			}
			if preference == "usdc" && strings.Contains(net, "eip155") {
				return r
			}
		}
		return x402.DefaultPaymentSelector(reqs)
	}

	clientCore := x402.Newx402Client(x402.WithPaymentSelector(selector))
	var finalPayerID string

	// 1. Check for EVM Identity
	if privKey, err := keyring.Get(config.KeyringService, config.UserAccount+"-key"); err == nil {
		signer, _ := evmsigners.NewClientSignerFromPrivateKey(privKey)
		clientCore.Register("eip155:*", evm.NewExactEvmScheme(signer))
		finalPayerID = signer.Address()
	}

	// 2. Check for Monero Identity
	// We'll store the primary address in the keyring during 'entropy login xmr'
	if xmrAddr, err := keyring.Get(config.KeyringService, config.UserAccount+"-xmr-addr"); err == nil {
		rpcURL, _ := keyring.Get(config.KeyringService, config.UserAccount+"-xmr-rpc")
		if rpcURL == "" {
			rpcURL = config.DefaultMoneroRPC
		}

		clientCore.Register("monero:*", &MoneroClientScheme{RPCURL: rpcURL})

		// If we don't have an EVM address, use the derived Monero ID
		if finalPayerID == "" {
			finalPayerID = DeriveMoneroID(xmrAddr)
		}
	}

	if finalPayerID == "" {
		return nil, fmt.Errorf("no identity linked: run 'entropy login' first")
	}

	wrappedClient := x402http.WrapHTTPClientWithPayment(
		&http.Client{Timeout: 150 * time.Second},
		x402http.Newx402HTTPClient(clientCore),
	)

	return &Client{
		HTTPClient: wrappedClient,
		PayerID:    finalPayerID,
	}, nil
}

// DoRequest is a helper to perform requests with standard Entropy headers
func (c *Client) DoRequest(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	fullURL := config.BaseURL + path

	var req *http.Request
	var err error

	if body != nil {
		bodyBytes, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	} else {
		req, err = http.NewRequestWithContext(ctx, method, fullURL, http.NoBody)
		if err != nil {
			return nil, err
		}
	}

	req.Header.Set("User-Agent", "Entropy-CLI/1.0")
	req.Header.Set("X-VM-PAYER", c.PayerID)

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.HTTPClient.Do(req)
}
