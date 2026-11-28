package ledger

// These names let the external tests of this package reach what the API returns and a few helpers.
type (
	// SummaryDTO exposes summaryDTO to the tests.
	SummaryDTO = summaryDTO
	// CategoryDTO exposes categoryDTO to the tests.
	CategoryDTO = categoryDTO
	// PaymentMethodDTO exposes paymentMethodDTO to the tests.
	PaymentMethodDTO = paymentMethodDTO
	// RecurrenceDTO exposes recurrenceDTO to the tests.
	RecurrenceDTO = recurrenceDTO
	// SettlementDTO exposes settlementDTO to the tests.
	SettlementDTO = settlementDTO
	// BudgetDTO exposes budgetDTO to the tests.
	BudgetDTO = budgetDTO
)

// MonthRange exposes monthRange to the tests.
var MonthRange = monthRange

// MustDate exposes mustDate to the tests.
var MustDate = mustDate
