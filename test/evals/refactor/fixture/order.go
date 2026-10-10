package shop

import (
	"errors"
	"fmt"
	"strings"
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
	e := strings.ToLower(strings.TrimSpace(receiptEmail))
	at := strings.LastIndexByte(e, '@')
	if at < 1 || at == len(e)-1 {
		return Order{}, errors.New("invalid email: " + e)
	}
	domain := e[at+1:]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return Order{}, errors.New("invalid email: " + e)
	}
	if strings.ContainsAny(e, " \t,;") {
		return Order{}, errors.New("invalid email: " + e)
	}
	return Order{ID: id, ReceiptEmail: e, Total: total}, nil
}
