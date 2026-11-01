package advernue

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Money is an integer-micros amount (1 unit = 1,000,000 micros) with an
// ISO-4217 currency. No floating point is ever involved.
type Money struct {
	Micros   int64  `json:"micros"`
	Currency string `json:"currency"`
}

// ParseMoney parses a decimal string such as "12.5" or "-0.000001" (at most
// six fractional digits) into Money.
func ParseMoney(decimal, currency string) (Money, error) {
	s := strings.TrimSpace(decimal)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" && frac == "" || len(frac) > 6 {
		return Money{}, fmt.Errorf("advernue: invalid decimal amount %q", decimal)
	}
	if whole == "" {
		whole = "0"
	}
	frac += strings.Repeat("0", 6-len(frac))
	w, err := strconv.ParseUint(whole, 10, 63)
	if err != nil {
		return Money{}, fmt.Errorf("advernue: invalid decimal amount %q", decimal)
	}
	f, err := strconv.ParseUint(frac, 10, 63)
	if err != nil || w > (1<<63-1-f)/1_000_000 {
		return Money{}, fmt.Errorf("advernue: invalid decimal amount %q", decimal)
	}
	m := int64(w*1_000_000 + f)
	if neg {
		m = -m
	}
	return Money{Micros: m, Currency: currency}, nil
}

// MustMoney is ParseMoney that panics; handy for constants.
func MustMoney(decimal, currency string) Money {
	m, err := ParseMoney(decimal, currency)
	if err != nil {
		panic(err)
	}
	return m
}

// Decimal renders the amount as a decimal string with at least two and at
// most six fractional digits, e.g. 12_500_000 -> "12.50".
func (m Money) Decimal() string {
	u := uint64(m.Micros)
	sign := ""
	if m.Micros < 0 {
		sign, u = "-", -u
	}
	frac := strings.TrimRight(fmt.Sprintf("%06d", u%1_000_000), "0")
	for len(frac) < 2 {
		frac += "0"
	}
	return fmt.Sprintf("%s%d.%s", sign, u/1_000_000, frac)
}

func (m Money) String() string { return m.Decimal() + " " + m.Currency }

// Add returns m+o; the currencies must match.
func (m Money) Add(o Money) (Money, error) {
	if m.Currency != o.Currency {
		return Money{}, errors.New("advernue: currency mismatch")
	}
	return Money{Micros: m.Micros + o.Micros, Currency: m.Currency}, nil
}
