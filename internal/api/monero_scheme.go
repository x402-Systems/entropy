package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	x402 "github.com/coinbase/x402/go"
)

type MoneroClientScheme struct {
	RPCURL string
}

func (s *MoneroClientScheme) Scheme() string {
	return "exact"
}

func (s *MoneroClientScheme) CreatePaymentPayload(ctx context.Context, req x402.PaymentRequirements) (x402.PaymentPayload, error) {
	amount, _ := strconv.ParseUint(req.Amount, 10, 64)

	// Construct the Monero RPC transfer request
	rpcPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "0",
		"method":  "transfer",
		"params": map[string]interface{}{
			"destinations": []map[string]interface{}{
				{"amount": amount, "address": req.PayTo},
			},
			"get_tx_key": true,
		},
	}

	body, _ := json.Marshal(rpcPayload)
	resp, err := http.Post(s.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return x402.PaymentPayload{}, fmt.Errorf("monero-wallet-rpc unreachable: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result struct {
			TxHash string `json:"tx_hash"`
			TxKey  string `json:"tx_key"`
		} `json:"result"`
		Error interface{} `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&rpcResp)

	if rpcResp.Error != nil {
		return x402.PaymentPayload{}, fmt.Errorf("xmr transfer failed: %v", rpcResp.Error)
	}

	return x402.PaymentPayload{
		X402Version: 2,
		Payload: map[string]interface{}{
			"address": req.PayTo,
			"tx_id":   rpcResp.Result.TxHash,
			"tx_key":  rpcResp.Result.TxKey,
		},
		Accepted: req,
	}, nil
}
