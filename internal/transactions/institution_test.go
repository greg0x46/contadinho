package transactions

import "testing"

func TestSanitizeInstitution(t *testing.T) {
	cases := []struct {
		name string
		raw  *string
		want *string
	}{
		{"nil stays nil", nil, nil},
		{"real institution passes through", strPtr("Nu Pagamentos S.A. - Instituição de Pagamento"), strPtr("Nu Pagamentos S.A. - Instituição de Pagamento")},
		{"proxy connector name is cleared", strPtr("MeuPluggy"), nil},
		{"proxy connector name with space is cleared", strPtr("Meu Pluggy"), nil},
		{"proxy connector name is case-insensitive", strPtr("meupluggy"), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeInstitution(tc.raw)
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Errorf("SanitizeInstitution(%v) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
