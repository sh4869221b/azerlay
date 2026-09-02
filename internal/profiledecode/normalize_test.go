package profiledecode

import "testing"

func TestNormalizeOuterText_WhenApprovedWrapperIsComplete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "surrounding whitespace", text: " \n payload \t", want: "payload"},
		{name: "leading BOM", text: " \n\ufeffpayload\t", want: "payload"},
		{name: "complete fence", text: "```\n payload \n```", want: "payload"},
		{name: "single quote", text: "' payload '", want: "payload"},
		{name: "double quote", text: `" payload "`, want: "payload"},
		{name: "triple single quote", text: "''' payload '''", want: "payload"},
		{name: "triple double quote", text: `""" payload """`, want: "payload"},
		{name: "fence then quote", text: "```\n' payload '\n```", want: "payload"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeOuterText(tt.text)

			if err != nil {
				t.Fatalf("normalizeOuterText() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeOuterText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeOuterText_WhenWrapperIsMalformed(t *testing.T) {
	t.Parallel()

	tests := []string{
		"```",
		"```\npayload",
		"payload\n```",
		"'",
		`"`,
		"'''",
		`"""`,
		"'payload",
		"payload'",
		`"payload`,
		`payload"`,
		`'payload"`,
		`'''payload"""`,
	}

	for _, text := range tests {
		text := text
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeOuterText(text)

			if err == nil {
				t.Fatal("normalizeOuterText() error = nil")
			}
		})
	}
}

func TestNormalizeOuterText_WhenContentContainsWhitespaceAndSymbols(t *testing.T) {
	t.Parallel()

	text := " \n{\"profiles\":[  ],\"note\":\"a`b # + / _\",\"bom\":\"\ufeff\"}\t "
	want := "{\"profiles\":[  ],\"note\":\"a`b # + / _\",\"bom\":\"\ufeff\"}"

	got, err := normalizeOuterText(text)

	if err != nil {
		t.Fatalf("normalizeOuterText() error = %v", err)
	}
	if got != want {
		t.Fatalf("normalizeOuterText() = %q, want %q", got, want)
	}
}

func TestNormalizeOuterText_WhenWrappersAreNested_RemovesOnlyOneQuoteLayer(t *testing.T) {
	t.Parallel()

	text := `"'{"profiles":[]}'"`

	got, err := normalizeOuterText(text)

	if err != nil {
		t.Fatalf("normalizeOuterText() error = %v", err)
	}
	if got != `'{"profiles":[]}'` {
		t.Fatalf("normalizeOuterText() = %q, want inner single-quote wrapper", got)
	}
}

func TestNormalizeOuterText_WhenBOMIsNotLeading_PreservesIt(t *testing.T) {
	t.Parallel()

	got, err := normalizeOuterText("payload\ufefftail")

	if err != nil {
		t.Fatalf("normalizeOuterText() error = %v", err)
	}
	if got != "payload\ufefftail" {
		t.Fatalf("normalizeOuterText() = %q", got)
	}
}
