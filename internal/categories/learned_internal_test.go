package categories

import "testing"

func TestSimilarityKey(t *testing.T) {
	cases := map[string]string{
		"Jim.Com* 50450362 Kau 3/12":  "jim.com* 50450362 kau",
		"Jim.Com* 50450362 Kau 12/12": "jim.com* 50450362 kau",
		"  PADARIA   RV  ":             "padaria rv",
		"Padaria Rv":                   "padaria rv",
		"Transferência enviada|Fulano": "transferência enviada|fulano",
		"Uber* Trip":                   "uber* trip",
		// A bare "N/M" with no preceding text and a date-like token are
		// not installment suffixes.
		"3/12":         "3/12",
		"Loja 2024/12": "loja 2024/12",
		"":             "",
	}
	for in, want := range cases {
		if got := similarityKey(in); got != want {
			t.Errorf("similarityKey(%q) = %q, want %q", in, got, want)
		}
	}
}
