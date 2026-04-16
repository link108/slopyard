package domain

import "testing"

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		host        string
		registrable string
	}{
		{
			name:        "full url",
			input:       "https://www.Example.com:443/path?x=1#top",
			host:        "example.com",
			registrable: "example.com",
		},
		{
			name:        "subdomain",
			input:       "blog.example.co.uk/articles",
			host:        "blog.example.co.uk",
			registrable: "example.co.uk",
		},
		{
			name:        "idn",
			input:       "https://bücher.example",
			host:        "xn--bcher-kva.example",
			registrable: "xn--bcher-kva.example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeHost(tt.input)
			if err != nil {
				t.Fatalf("NormalizeHost returned error: %v", err)
			}
			if got.Host != tt.host {
				t.Fatalf("host = %q, want %q", got.Host, tt.host)
			}
			if got.RegistrableDomain != tt.registrable {
				t.Fatalf("registrable = %q, want %q", got.RegistrableDomain, tt.registrable)
			}
		})
	}
}

func TestNormalizeHostRejectsInvalidInput(t *testing.T) {
	tests := []string{"", "http://", "127.0.0.1", "https://[::1]/"}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := NormalizeHost(input); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
