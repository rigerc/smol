package format //nolint:revive // Tests share the display formatting package name, distinct from go/format.

import "testing"

func TestFormatting(t *testing.T) {
	o := Default()
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		bytes   uint64
		options SizeOptions
		want    string
	}{{512 * 1024, o.Size, "512 KiB"}, {1024 * 1024, o.Size, "1 MiB"}, {1024 * 1024 * 1024, o.Size, "1.0 GiB"}, {1000000, SizeOptions{"si", 2}, "1.00 MB"}, {1024 * 1024, SizeOptions{"iec", 1}, "1.0 MiB"}} {
		if got := Size(tc.bytes, tc.options); got != tc.want {
			t.Errorf("size %q want %q", got, tc.want)
		}
	}
	for _, tc := range []struct {
		seconds int64
		options DurationOptions
		want    string
	}{{59, o.Duration, "<1m"}, {86460, o.Duration, "1d 0h 1m"}, {86460, DurationOptions{Abbreviation: true, Compact: true}, "1d 1m"}, {90, DurationOptions{Abbreviation: true, Round: true, SpaceBeforeUnit: true}, "2 m"}, {86460, DurationOptions{}, "1 day 0 hours 1 minute"}, {0, DurationOptions{}, "<1 minute"}} {
		if got := Duration(tc.seconds, tc.options); got != tc.want {
			t.Errorf("duration %q want %q", got, tc.want)
		}
	}
	for _, v := range []float64{-1, 0, 50, 100, 200} {
		pct, bar := Percent(v, o.Percent)
		if pct == "" || len(bar) != 12 {
			t.Fatal(pct, bar)
		}
	}
	for _, tc := range []struct {
		o    TempOptions
		want string
	}{{TempOptions{"c", 0}, "0°C"}, {TempOptions{"f", 0}, "32°F"}, {TempOptions{"k", 2}, "273.15K"}} {
		if got := Temperature(0, tc.o); got != tc.want {
			t.Fatal(got)
		}
	}
	for _, change := range []func(*Options){func(o *Options) { o.Size.Unit = "bad" }, func(o *Options) { o.Temp.Unit = "bad" }, func(o *Options) { o.Size.Digits = -2 }, func(o *Options) { o.Percent.Width = 0 }, func(o *Options) { o.Temp.Digits = 7 }} {
		bad := Default()
		change(&bad)
		if bad.Validate() == nil {
			t.Fatal(bad)
		}
	}
}
