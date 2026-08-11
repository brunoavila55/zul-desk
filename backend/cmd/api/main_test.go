package main

import "testing"

func TestDigits(t *testing.T) {
	t.Parallel()
	if got := digits("+55 (55) 99999-9999"); got != "5555999999999" {
		t.Fatalf("digits() = %q", got)
	}
}
