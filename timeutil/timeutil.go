// Package timeutil provides a single, explicit source of truth for the
// business timezone (Thailand, UTC+7, no DST). Every place in the codebase
// that needs to interpret or display a "wall clock" time-of-day value
// (attendance schedules, check-in/check-out) must go through BangkokLocation
// instead of relying on the ambient timezone of the machine, DB session, or
// browser it happens to run on — that ambient-timezone dependence is what
// caused schedule times to read back shifted by several hours depending on
// where the code ran.
package timeutil

import "time"

// BangkokLocation is a fixed +7:00 offset (Thailand observes no DST), so it
// never depends on the host having an IANA timezone database installed.
var BangkokLocation = time.FixedZone("Asia/Bangkok", 7*60*60)

// ToBangkok converts t to the business timezone without changing the
// instant it represents.
func ToBangkok(t time.Time) time.Time {
	return t.In(BangkokLocation)
}

// DayStart returns the start (00:00) of t's calendar day in the business
// timezone, as an absolute instant anchored to BangkokLocation. Use this
// instead of t.Truncate(24*time.Hour), which aligns to UTC day boundaries
// and can land on the wrong Bangkok calendar day.
func DayStart(t time.Time) time.Time {
	b := ToBangkok(t)
	return time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, BangkokLocation)
}

// DateKey returns t's calendar date in the business timezone as "2006-01-02".
func DateKey(t time.Time) string {
	return DayStart(t).Format("2006-01-02")
}

// ParseDayStart reads a "2006-01-02" string as midnight of that date in the
// business timezone. Use it when the parsed value is compared against, or used to
// build, a real timestamp — time.Parse would yield UTC midnight, which is 07:00
// Bangkok, so a day-long window built from it straddles two Bangkok dates.
//
// Do NOT use this for the date-only columns that deliberately carry no timezone
// (Holiday.Date, PayrollCycle.Start/EndDate, LeaveRequest.Start/EndDate,
// SalariesStructure.EffectiveDate). Those are compared against other date-only
// values, and anchoring them to +07 would shift them.
func ParseDayStart(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, BangkokLocation)
}
