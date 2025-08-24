package swap

import (
	"github.com/oklog/ulid/v2"
	"github.com/robaho/fixed"
)

type Swap struct {
	ulid ulid.ULID

	data Data // immutable
}

func (sw *Swap) ULID() ulid.ULID     { return sw.ulid }
func (sw *Swap) Who() string         { return sw.data.Who }
func (sw *Swap) Token() string       { return sw.data.Token }
func (sw *Swap) Amount() fixed.Fixed { return sw.data.Amount }
func (sw *Swap) USD() fixed.Fixed    { return sw.data.USD }
func (sw *Swap) Side() bool          { return sw.data.Side }

type Data struct {
	Who    string
	Token  string
	Amount fixed.Fixed
	USD    fixed.Fixed
	Side   bool
}

func New(d Data) Swap {
	return Swap{
		ulid: ulid.Make(),
		data: d,
	}
}

func Reconstruct(ulid ulid.ULID, d Data) Swap {
	return Swap{
		ulid: ulid,
		data: d,
	}
}
