package vm

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"swaps/internal/swap"
)

type VM struct {
	csvImportURL string
	queryURL     string
	cli          *http.Client
}

func New(url string) *VM {
	return &VM{
		queryURL:     url + "/api/v1/query_range",
		csvImportURL: url + "/api/v1/import/csv?format=" + vmCSVFormat,
		cli:          &http.Client{},
	}
}

func (vm *VM) PushBatch(ctx context.Context, in []swap.Swap) error {
	var b bytes.Buffer

	w := csv.NewWriter(&b)
	for _, sw := range in {
		err := w.Write([]string{
			strconv.Itoa(int(sw.ULID().Timestamp().UnixMilli())),
			sw.Token(),
			sw.Amount().String(),
			sw.USD().String(),
		})
		if err != nil {
			return fmt.Errorf("csv write: %w", err)
		}
	}
	w.Flush()

	req, err := http.NewRequestWithContext(ctx, "POST", vm.csvImportURL, &b)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	resp, err := vm.cli.Do(req)
	if err != nil {
		return fmt.Errorf("http to vm: %w", err)
	}
	defer resp.Body.Close() //nolint

	return nil
}

const vmCSVFormat = `1:time:unix_ms,2:label:token,3:metric:amount,4:metric:amount_usd`
