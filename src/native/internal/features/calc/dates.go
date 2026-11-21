package calc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Day arithmetic is refused beyond these bounds, which match the JavaScript Date range the original relied on.
const (
	maxDayOffset      = 1e9
	maxEpochMillis    = 8.64e15
	millisPerDay      = 86400000
	isoDateLayout     = "2006-01-02"
	displayDateLayout = "Jan 2, 2006"
)

var (
	dayFirstPattern   = regexp.MustCompile(`(?i)^(days?\s+until\s+)?(\d{1,2})\s+([a-z]+)$`)
	monthFirstPattern = regexp.MustCompile(`(?i)^(days?\s+until\s+)?([a-z]+)\s+(\d{1,2})$`)
	tomorrowPattern   = regexp.MustCompile(`(?i)^tomorrow$`)
	yesterdayPattern  = regexp.MustCompile(`(?i)^yesterday$`)
	daysAgoPattern    = regexp.MustCompile(`(?i)^(\d+)\s+days?\s+ago$`)
	weekdayPattern    = regexp.MustCompile(`(?i)^(?:next\s+)?([a-z]+)(?:\s+in\s+(\d+)\s+weeks?)?$`)
)

// weekdayIndex maps weekday names and abbreviations to their Go weekday.
var weekdayIndex = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

// monthIndex maps month names and abbreviations to their Go month.
var monthIndex = map[string]time.Month{
	"jan": time.January, "january": time.January,
	"feb": time.February, "february": time.February,
	"mar": time.March, "march": time.March,
	"apr": time.April, "april": time.April,
	"may": time.May,
	"jun": time.June, "june": time.June,
	"jul": time.July, "july": time.July,
	"aug": time.August, "august": time.August,
	"sep": time.September, "sept": time.September, "september": time.September,
	"oct": time.October, "october": time.October,
	"nov": time.November, "november": time.November,
	"dec": time.December, "december": time.December,
}

// monthDayQuery is a "25 Dec" or "December 25" question, optionally asking for the days until it.
type monthDayQuery struct {
	until bool
	day   int
	month string
}

// dateCalendar carries the instant and zone that relative dates are computed from.
type dateCalendar struct {
	now  time.Time
	zone *time.Location
}

// today is local midnight of the calendar day containing now.
func (c dateCalendar) today() time.Time {
	local := c.now.In(c.zone)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, c.zone)
}

// calculateDate answers calendar questions: month and day, tomorrow, yesterday, "N days ago", and weekdays with an
// optional "in N weeks". It returns false when the expression is not a date question.
func calculateDate(expression, query string, calendar dateCalendar) (Result, bool) {
	trimmed := strings.TrimSpace(expression)
	if monthDay, ok := parseMonthDay(trimmed); ok {
		return monthDayResult(monthDay, expression, query, calendar)
	}
	if tomorrowPattern.MatchString(trimmed) {
		return dateOffsetResult(expression, query, calendar, 1)
	}
	if yesterdayPattern.MatchString(trimmed) {
		return dateOffsetResult(expression, query, calendar, -1)
	}
	if groups := daysAgoPattern.FindStringSubmatch(trimmed); groups != nil {
		days, ok := parseDayCount(groups[1])
		if !ok {
			return Result{}, false
		}
		return dateOffsetResult(expression, query, calendar, -days)
	}
	return weekdayResult(trimmed, expression, query, calendar)
}

// parseMonthDay recognizes "25 Dec", "December 25" and the "days until" forms of both.
func parseMonthDay(trimmed string) (monthDayQuery, bool) {
	if groups := dayFirstPattern.FindStringSubmatch(trimmed); groups != nil {
		return monthDayQuery{until: groups[1] != "", day: parseSmallInt(groups[2]), month: groups[3]}, true
	}
	if groups := monthFirstPattern.FindStringSubmatch(trimmed); groups != nil {
		return monthDayQuery{until: groups[1] != "", day: parseSmallInt(groups[3]), month: groups[2]}, true
	}
	return monthDayQuery{}, false
}

// monthDayResult answers a month and day question with the date or, when asked, the days until it.
func monthDayResult(question monthDayQuery, expression, query string, calendar dateCalendar) (Result, bool) {
	today := calendar.today()
	target, ok := nextMonthDay(question.day, question.month, today, calendar.zone)
	if !ok {
		return Result{}, false
	}
	if !question.until {
		return dateResult(query, expression, target, "Date"), true
	}
	days := jsRound(float64(target.Sub(today).Milliseconds()) / millisPerDay)
	text := fmt.Sprintf("%s %s", integerText(days), pluralDays(days))
	r := newResult(KindDate, strings.TrimSpace(query), strings.TrimSpace(expression), days, text, text, "Until "+formatDateDisplay(target))
	r.Date = text
	return describe(r), true
}

// pluralDays names a count of days.
func pluralDays(days float64) string {
	if days == 1 {
		return "day"
	}
	return "days"
}

// nextMonthDay finds the next occurrence of a month and day on or after today, rolling to next year when it has
// passed. Dates that do not exist, such as 31 February, are refused.
func nextMonthDay(day int, monthName string, today time.Time, zone *time.Location) (time.Time, bool) {
	month, ok := monthIndex[strings.ToLower(monthName)]
	if !ok || day < 1 || day > 31 {
		return time.Time{}, false
	}
	date := time.Date(today.Year(), month, day, 0, 0, 0, 0, zone)
	if date.Month() != month {
		return time.Time{}, false
	}
	if date.Before(today) {
		date = time.Date(today.Year()+1, month, day, 0, 0, 0, 0, zone)
	}
	return date, true
}

// dateOffsetResult answers "tomorrow", "yesterday" and "N days ago" with the date that many days from today.
func dateOffsetResult(expression, query string, calendar dateCalendar, offset float64) (Result, bool) {
	date, ok := addLocalDays(calendar.today(), offset, calendar.zone)
	if !ok {
		return Result{}, false
	}
	return dateResult(query, expression, date, "Date"), true
}

// weekdayResult answers "monday", "next monday" and "monday in 3 weeks".
func weekdayResult(trimmed, expression, query string, calendar dateCalendar) (Result, bool) {
	groups := weekdayPattern.FindStringSubmatch(trimmed)
	if groups == nil {
		return Result{}, false
	}
	weekday, ok := weekdayIndex[strings.ToLower(groups[1])]
	if !ok {
		return Result{}, false
	}
	weeks := 0.0
	if groups[2] != "" {
		parsed, ok := parseDayCount(groups[2])
		if !ok {
			return Result{}, false
		}
		weeks = parsed
	}
	today := calendar.today()
	date, ok := addLocalDays(nextWeekday(today, weekday, calendar.zone), weeks*7, calendar.zone)
	if !ok {
		return Result{}, false
	}
	return dateResult(query, expression, date, "Date"), true
}

// parseDayCount reads a count of days or weeks, refusing values beyond the calendar range.
func parseDayCount(digits string) (float64, bool) {
	value, err := strconv.ParseFloat(digits, 64)
	if err != nil || value > maxDayOffset {
		return 0, false
	}
	return value, true
}

// nextWeekday is the first date after today that falls on the weekday, so today's own weekday means a week away.
func nextWeekday(today time.Time, weekday time.Weekday, zone *time.Location) time.Time {
	delta := (int(weekday) - int(today.Weekday()) + 7) % 7
	if delta == 0 {
		delta = 7
	}
	date, _ := addLocalDays(today, float64(delta), zone)
	return date
}

// addLocalDays moves a local midnight by whole days. It fails when the result leaves the calendar range.
func addLocalDays(date time.Time, days float64, zone *time.Location) (time.Time, bool) {
	if days > maxDayOffset || days < -maxDayOffset {
		return time.Time{}, false
	}
	next := time.Date(date.Year(), date.Month(), date.Day()+int(days), 0, 0, 0, 0, zone)
	millis := float64(next.UnixMilli())
	if millis > maxEpochMillis || millis < -maxEpochMillis {
		return time.Time{}, false
	}
	return next, true
}

// dateResult builds a date answer whose raw text is the ISO date and whose display is the readable date.
func dateResult(query, expression string, date time.Time, subtitle string) Result {
	raw := date.Format(isoDateLayout)
	display := formatDateDisplay(date)
	r := newResult(KindDate, strings.TrimSpace(query), strings.TrimSpace(expression), float64(date.UnixMilli()), raw, display, subtitle)
	r.Date = raw
	return describe(r)
}

// formatDateDisplay renders a date the way the en-US short form reads, such as "Dec 25, 2026".
func formatDateDisplay(date time.Time) string {
	return date.Format(displayDateLayout)
}
