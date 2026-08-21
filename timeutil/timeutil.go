// Package timeutil provides a single, explicit source of truth for the business timezone (Thailand, UTC+7, no DST).
package timeutil

import "time"

// BangkokLocation is a fixed +7:00 offset (Thailand observes no DST), so it never depends on the host having an IANA timezone database installed.
var BangkokLocation = time.FixedZone("Asia/Bangkok", 7*60*60)

// ToBangkok converts t to the business timezone without changing the instant it represents.
func ToBangkok(t time.Time) time.Time {
	return t.In(BangkokLocation)
}

// DayStart returns the start (00:00) of t's calendar day in the business timezone, as an absolute instant anchored to BangkokLocation.
func DayStart(t time.Time) time.Time {
	b := ToBangkok(t)
	return time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, BangkokLocation)
}

// DateKey returns t's calendar date in the business timezone as "2006-01-02".
func DateKey(t time.Time) string {
	return DayStart(t).Format("2006-01-02")
}

// ParseDayStart reads a "2006-01-02" string as midnight of that date in the business timezone.
func ParseDayStart(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, BangkokLocation)
}
