package storage

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/go-pg/pg/v10"
	"github.com/oklog/ulid/v2"
	"github.com/robaho/fixed"
	"swaps/internal/swap"
)

type Storage struct {
	db *pg.DB
}

func New(db *pg.DB) *Storage {
	return &Storage{db: db}
}

func (st *Storage) Persist(ctx context.Context, sw swap.Swap) error {
	params := struct {
		ULID   string `pg:"ulid"`
		Who    string `pg:"who"`
		Token  string `pg:"token"`
		Amount string `pg:"amount"`
		USD    string `pg:"usd"`
		Side   bool   `pg:"side,use_zero"`
	}{
		ULID:   sw.ULID().String(),
		Who:    sw.Who(),
		Token:  sw.Token(),
		Amount: sw.Amount().String(),
		USD:    sw.USD().String(),
		Side:   sw.Side(),
	}
	_, err := st.db.ExecContext(ctx, persistSwapQ, &params)
	return err
}

func (st *Storage) ProcessSwaps(ctx context.Context, f func(swaps []swap.Swap) (int, error)) (int, error) {
	var total int
	err := st.db.RunInTransaction(ctx, func(tx *pg.Tx) error {
		var locked []LockedSwap
		if _, err := tx.QueryContext(ctx, &locked, processSwapQ); err != nil {
			return fmt.Errorf("lock: %w", err)
		}

		swaps := make([]swap.Swap, 0, len(locked))
		ids := make([]int64, 0, len(locked))
		for _, ls := range locked {
			sw := swap.Reconstruct(ulid.MustParse(ls.ULID), swap.Data{
				Who:    ls.Who,
				Token:  ls.Token,
				Amount: fixed.MustParse(ls.Amount),
				USD:    fixed.MustParse(ls.USD),
				Side:   ls.Side,
			})
			swaps = append(swaps, sw)
			ids = append(ids, ls.OutboxID)
		}

		n, err := f(swaps)
		if err != nil {
			return fmt.Errorf("process: %w", err)
		}
		total = n

		delParams := struct {
			Ids []int64 `pg:"ids,array"`
		}{
			Ids: ids,
		}

		if _, err := tx.ExecContext(ctx, deleteQ, delParams); err != nil {
			return fmt.Errorf("delete: %w", err)
		}

		return nil
	})

	return total, err
}

type LockedSwap struct {
	ULID     string `pg:"ulid"`
	OutboxID int64  `pg:"id"`

	Who    string `pg:"who"`
	Token  string `pg:"token"`
	Amount string `pg:"amount"`
	USD    string `pg:"usd"`
	Side   bool   `pg:"side"`
}

//go:embed sql/process_swaps.sql
var processSwapQ string

const persistSwapQ = `insert into swap(ulid, who, token, amount, usd, side) 
values (?ulid, ?who, ?token, ?amount, ?usd, ?side);`

const deleteQ = `delete from swap_outbox where id = any(?ids);`
