package advernue

import "testing"

func TestMoney(t *testing.T) {
	cases := []struct {
		in     string
		micros int64
		out    string
	}{
		{"12.5", 12_500_000, "12.50"},
		{"0.000001", 1, "0.000001"},
		{"-3", -3_000_000, "-3.00"},
		{".25", 250_000, "0.25"},
		{"1.123456", 1_123_456, "1.123456"},
	}
	for _, c := range cases {
		m, err := ParseMoney(c.in, "USD")
		if err != nil || m.Micros != c.micros || m.Decimal() != c.out {
			t.Errorf("%s: got %v %v %s", c.in, m, err, m.Decimal())
		}
	}
	for _, bad := range []string{"", "abc", "1.1234567", "1.2.3", "99999999999999999999", "-"} {
		if _, err := ParseMoney(bad, "USD"); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
	a, _ := ParseMoney("1.10", "USD")
	b, _ := ParseMoney("2.20", "USD")
	if s, _ := a.Add(b); s.Decimal() != "3.30" {
		t.Errorf("add: %v", s)
	}
	if _, err := a.Add(Money{Currency: "EUR"}); err == nil {
		t.Error("currency mismatch should fail")
	}
}
