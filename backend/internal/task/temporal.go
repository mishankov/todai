package task

import (
	"database/sql/driver"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

const timeOfDayLayout = "15:04"

// Date is a calendar date without a time or timezone.
type Date string

// ParseDate parses the API representation of a calendar date.
func ParseDate(value string) (Date, error) {
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return "", fmt.Errorf("parse date: %w", err)
	}

	return Date(parsed.Format(dateLayout)), nil
}

// Scan reads a PostgreSQL DATE value.
func (d *Date) Scan(value any) error {
	switch value := value.(type) {
	case time.Time:
		*d = Date(value.Format(dateLayout))
		return nil
	case string:
		parsed, err := ParseDate(value)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case []byte:
		return d.Scan(string(value))
	default:
		return fmt.Errorf("scan date from %T", value)
	}
}

// Value writes a PostgreSQL DATE value.
func (d Date) Value() (driver.Value, error) {
	return string(d), nil
}

// TimeOfDay is a local wall-clock time with minute precision.
type TimeOfDay string

// ParseTimeOfDay parses the API representation of a local time.
func ParseTimeOfDay(value string) (TimeOfDay, error) {
	parsed, err := time.Parse(timeOfDayLayout, value)
	if err != nil {
		return "", fmt.Errorf("parse time of day: %w", err)
	}

	return TimeOfDay(parsed.Format(timeOfDayLayout)), nil
}

// Scan reads a PostgreSQL TIME value.
func (t *TimeOfDay) Scan(value any) error {
	switch value := value.(type) {
	case time.Time:
		*t = TimeOfDay(value.Format(timeOfDayLayout))
		return nil
	case string:
		parsed, err := time.Parse("15:04:05", value)
		if err != nil {
			return fmt.Errorf("parse database time of day: %w", err)
		}
		*t = TimeOfDay(parsed.Format(timeOfDayLayout))
		return nil
	case []byte:
		return t.Scan(string(value))
	default:
		return fmt.Errorf("scan time of day from %T", value)
	}
}

// Value writes a PostgreSQL TIME value.
func (t TimeOfDay) Value() (driver.Value, error) {
	return string(t), nil
}
