package main

import "testing"

func TestDigits(t *testing.T) {
	t.Parallel()
	if got := digits("+55 (55) 99999-9999"); got != "5555999999999" {
		t.Fatalf("digits() = %q", got)
	}
}

func TestTemplateParameterCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "sem variáveis", content: "Hello World", want: 0},
		{name: "três variáveis", content: "Olá {{1}}, aqui é {{2}}. Você é cliente há {{3}}.", want: 3},
		{name: "variável repetida", content: "Olá {{1}}. Até logo, {{1}}.", want: 1},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := templateParameterCount(test.content); got != test.want {
				t.Fatalf("templateParameterCount() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestLoginAttemptLimit(t *testing.T) {
	t.Parallel()
	a := &app{loginAttempts: map[string]loginAttempt{}}
	for i := 0; i < 10; i++ {
		if !a.allowLogin("192.0.2.10") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if a.allowLogin("192.0.2.10") {
		t.Fatal("eleventh attempt should be blocked")
	}
	a.clearLoginAttempts("192.0.2.10")
	if !a.allowLogin("192.0.2.10") {
		t.Fatal("successful login should reset the limiter")
	}
}
