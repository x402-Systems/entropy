package api

import (
	"context"
	"entropy/internal/config"
	"fmt"
	"io"
	"net/http"
	"time"

	x402 "github.com/coinbase/x402/go"
	x402http "github.com/coinbase/x402/go/http"
	evm "github.com/coinbase/x402/go/mechanisms/evm/exact/client"
	evmsigners "github.com/coinbase/x402/go/signers/evm"
	"github.com/zalando/go-keyring"
)

type Client struct {
	HTTPClient *http.Client
	Address    string
}

// NewClient initializes the x402 payment-wrapped HTTP client
func NewClient() (*Client, error) {
	privKey, err := keyring.Get(config.KeyringService, config.UserAccount+"-key")
	if err != nil {
		return nil, fmt.Errorf("wallet not linked: run 'entropy login' first")
	}

	signer, err := evmsigners.NewClientSignerFromPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key in keyring: %v", err)
	}

	clientCore := x402.Newx402Client()
	clientCore.Register("eip155:*", evm.NewExactEvmScheme(signer))

	wrappedClient := x402http.WrapHTTPClientWithPayment(
		&http.Client{Timeout: 150 * time.Second},
		x402http.Newx402HTTPClient(clientCore),
	)

	return &Client{
		HTTPClient: wrappedClient,
		Address:    signer.Address(),
	}, nil
}

// DoRequest is a helper to perform requests with standard Entropy headers
func (c *Client) DoRequest(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	fullURL := config.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-VM-PAYER", c.Address)

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.HTTPClient.Do(req)
}
