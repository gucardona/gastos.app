// Package billing holds the credit card invoice rule: which invoice, and so which month, a purchase falls in.
package billing

import (
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

// InvoiceDue returns the due date (YYYY-MM-DD) of the invoice a purchase made on date falls in, for a card whose
// purchases up to closingDay (inclusive) close in that month's invoice and whose invoice is due on dueDay.
// A day past the end of a month is that month's last day.
func InvoiceDue(date string, closingDay, dueDay int) (string, error) {
	if closingDay < 1 || closingDay > 31 || dueDay < 1 || dueDay > 31 {
		return "", fmt.Errorf("invalid card days: closing %d, due %d", closingDay, dueDay)
	}
	p, err := time.Parse(dateLayout, date)
	if err != nil {
		return "", err
	}
	closing := firstOfMonth(p)
	if p.Day() > clamp(closing, closingDay) {
		closing = closing.AddDate(0, 1, 0)
	}
	due := closing
	if dueDay <= closingDay {
		due = due.AddDate(0, 1, 0)
	}
	return due.AddDate(0, 0, clamp(due, dueDay)-1).Format(dateLayout), nil
}

// BestPurchaseDay is the first day of a new invoice: the day after the closing (31 wraps to 1).
func BestPurchaseDay(closingDay int) int { return closingDay%31 + 1 }

func firstOfMonth(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC) }

// clamp returns day, or the last day of month's month when it has fewer days.
func clamp(month time.Time, day int) int {
	return min(day, firstOfMonth(month).AddDate(0, 1, -1).Day())
}
