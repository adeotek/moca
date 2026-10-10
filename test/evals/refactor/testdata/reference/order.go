package shop

import (
	"errors"
	"fmt"
)

type Order struct {
	ID           int
	ReceiptEmail string
	Total        float64
}

// NewOrder validates an order; the receipt goes to receiptEmail.
func NewOrder(id int, receiptEmail string, total float64) (Order, error) {
	if id <= 0 {
		return Order{}, fmt.Errorf("order id must be positive, got %d", id)
	}
	if total < 0 {
		return Order{}, errors.New("total must not be negative")
	}
	e, err := normalizeEmail(receiptEmail)
	if err != nil {
		return Order{}, err
	}
	return Order{ID: id, ReceiptEmail: e, Total: total}, nil
}
