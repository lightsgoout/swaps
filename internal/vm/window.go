package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func (vm *VM) GetWindow(ctx context.Context, token string, interval string) (Window, error) {
	v, err := vm.runQuery(ctx, fmt.Sprintf(`sum_over_time(amount{token="%s"}[%s])`, token, interval))
	if err != nil {
		return Window{}, fmt.Errorf("vol query: %w", err)
	}

	vUSD, err := vm.runQuery(ctx, fmt.Sprintf(`sum_over_time(amount_usd{token="%s"}[%s])`, token, interval))
	if err != nil {
		return Window{}, fmt.Errorf("volUSD query: %w", err)
	}

	txCount, err := vm.runQuery(ctx, fmt.Sprintf(`count_over_time(amount{token="%s"}[%s])`, token, interval))
	if err != nil {
		return Window{}, fmt.Errorf("txCount query: %w", err)
	}

	return Window{
		Volume:    v,
		VolumeUSD: vUSD,
		TxCount:   txCount,
	}, nil
}

func (vm *VM) runQuery(ctx context.Context, query string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", vm.queryURL, nil)
	if err != nil {
		return "", fmt.Errorf("vm new request: %w", err)
	}
	q := req.URL.Query()
	q.Add("query", query)
	req.URL.RawQuery = q.Encode()

	resp, err := vm.cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("http to vm: %w", err)
	}
	defer resp.Body.Close() //nolint

	var qr queryResponse

	err = json.NewDecoder(resp.Body).Decode(&qr)
	if err != nil {
		return "", fmt.Errorf("json: %w", err)
	}

	if qr.Status != "success" {
		return "", fmt.Errorf("vm error response")
	}

	if len(qr.Data.Result) == 0 {
		return "", nil
	}

	if len(qr.Data.Result[0].Values) == 0 {
		return "", nil
	}

	raw, ok := qr.Data.Result[0].Values[0][1].(string)
	if !ok {
		return "", fmt.Errorf("vm schema mismatch")
	}

	return raw, nil
}

type queryResponse struct {
	Status string `json:"status"`
	Data   data   `json:"data"`
}

type data struct {
	Result []entry `json:"result"`
}

type entry struct {
	Values [][]any `json:"values"`
}

type Window struct {
	Volume    string `json:"volume"`
	VolumeUSD string `json:"volume_usd"`
	TxCount   string `json:"tx_count"`
}
