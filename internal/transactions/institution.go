package transactions

import "strings"

// isPluggyProxyConnector reports whether a provider-reported "institution"
// actually names Pluggy's own "Meu Pluggy" connector (id 200) — a proxy
// layer over connections the user already granted elsewhere, not a real
// bank. Pluggy is the only provider this app integrates with, so the
// connector's own product name is the one placeholder worth filtering.
func isPluggyProxyConnector(name string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(name), ""))
	return normalized == "meupluggy"
}

// SanitizeInstitution clears a raw "institution" that is really Pluggy's
// proxy connector name rather than a financial institution, so it never
// reaches a user-facing list or filter. Everywhere financial_accounts.institution
// is exposed should go through this.
func SanitizeInstitution(raw *string) *string {
	if raw == nil || isPluggyProxyConnector(*raw) {
		return nil
	}
	return raw
}
