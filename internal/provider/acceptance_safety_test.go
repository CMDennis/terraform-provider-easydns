package provider

import "testing"

func TestValidateAcceptanceBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "sandbox", value: "https://sandbox.rest.easydns.net", want: "https://sandbox.rest.easydns.net"},
		{name: "sandbox default port", value: "https://sandbox.rest.easydns.net:443/", want: "https://sandbox.rest.easydns.net:443"},
		{name: "production", value: "https://rest.easydns.net", wantErr: true},
		{name: "lookalike", value: "https://sandbox.rest.easydns.net.example.invalid", wantErr: true},
		{name: "HTTP", value: "http://sandbox.rest.easydns.net", wantErr: true},
		{name: "custom port", value: "https://sandbox.rest.easydns.net:3001", wantErr: true},
		{name: "credentials", value: "https://user:pass@sandbox.rest.easydns.net", wantErr: true},
		{name: "path", value: "https://sandbox.rest.easydns.net/api", wantErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := validateAcceptanceBaseURL(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("error=%v, wantErr=%v", err, test.wantErr)
			}
			if !test.wantErr && got != test.want {
				t.Fatalf("got=%q, want=%q", got, test.want)
			}
		})
	}
}
