package shop

import "testing"

var emails = map[string]bool{
	"a@b.co":            true,
	" Ann@Example.ORG ": true,
	"@b.co":             false,
	"a@":                false,
	"a@b":               false,
	"a@.co":             false,
	"a@b.co.":           false,
	"a b@c.de":          false,
	"x@y@z.io":          true,
}

func TestUserEmail(t *testing.T) {
	for e, ok := range emails {
		_, err := NewUser("ann", e)
		if (err == nil) != ok {
			t.Errorf("NewUser email %q: err=%v, want ok=%v", e, err, ok)
		}
	}
	u, _ := NewUser(" Ann ", " Ann@Example.ORG ")
	if u.Name != "Ann" || u.Email != "ann@example.org" {
		t.Errorf("normalized: %+v", u)
	}
}

func TestOrderEmail(t *testing.T) {
	for e, ok := range emails {
		_, err := NewOrder(1, e, 10)
		if (err == nil) != ok {
			t.Errorf("NewOrder email %q: err=%v, want ok=%v", e, err, ok)
		}
	}
	if _, err := NewOrder(0, "a@b.co", 1); err == nil {
		t.Error("id 0 must fail")
	}
	if _, err := NewOrder(1, "a@b.co", -1); err == nil {
		t.Error("negative total must fail")
	}
}
