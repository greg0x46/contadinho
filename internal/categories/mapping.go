package categories

// SamePersonTransferLabel is Pluggy's label (categoryId 04000000) for a
// transfer between two accounts held by the same person — the provider
// already recognizes these by matching the payer's and receiver's tax id.
// Named because BackfillAutomatic has to address it explicitly.
const SamePersonTransferLabel = "Same person transfer"

// SourceCategoryMapping maps a provider's source_category label to the
// internal category it is automatically assigned to. Kept close to the
// Python reference's app/categories/mapping.py (and duplicated again in this
// project's 00004_categories.sql seed, per its own comment).
//
// A transfer between the user's own accounts is mapped here on purpose, to
// the seeded transfer-kind category: money.Eligibility drops a transfer-kind
// transaction out of income/expense totals (ReasonTransferCategory) while
// TotalsEligibility.MovesCash keeps it in balance reconstruction, which is
// exactly what "my own money changing banks" needs. Leaving it uncategorized
// does the opposite — no category at all is eligible, so the same money lands
// in the totals as income on one leg and expense on the other. That transfer
// kind did not exist when this map was first ported, which is why the label
// used to be excluded.
//
// The remaining self-directed labels ("Investments", "Third party
// transfers", "Credit card payment") are still left out: each one needs its
// own decision about which internal category it belongs to, and none of them
// is simply a transfer between the user's own accounts.
var SourceCategoryMapping = map[string]string{
	"Groceries":                   "000433b6-3094-5a9c-87df-465b70574a4b",
	"Alimentação":                 "12cdb9e7-3f28-5fcf-a675-a2195a732bf1",
	"Food":                        "12cdb9e7-3f28-5fcf-a675-a2195a732bf1",
	"Food and drinks":             "12cdb9e7-3f28-5fcf-a675-a2195a732bf1",
	"Eating out":                  "12cdb9e7-3f28-5fcf-a675-a2195a732bf1",
	"Food delivery":               "12cdb9e7-3f28-5fcf-a675-a2195a732bf1",
	"Taxi and ride-hailing":       "99f20e22-cd96-5081-afd0-068a203c5fde",
	"Gas stations":                "99f20e22-cd96-5081-afd0-068a203c5fde",
	"Parking":                     "99f20e22-cd96-5081-afd0-068a203c5fde",
	"Vehicle maintenance":         "99f20e22-cd96-5081-afd0-068a203c5fde",
	"Housing":                     "bbdc84e0-788a-59f7-a9f4-b2143172e53b",
	"Water":                       "bbdc84e0-788a-59f7-a9f4-b2143172e53b",
	"Telecommunications":          "aec4034c-f5a2-59a0-a1f9-1d737e4f1f3f",
	"Healthcare":                  "17f56080-837c-5ebb-a828-93fcb891bb8f",
	"Pharmacy":                    "17f56080-837c-5ebb-a828-93fcb891bb8f",
	"Hospital clinics and labs":   "17f56080-837c-5ebb-a828-93fcb891bb8f",
	"Optometry":                   "17f56080-837c-5ebb-a828-93fcb891bb8f",
	"Wellness and fitness":        "17f56080-837c-5ebb-a828-93fcb891bb8f",
	"Gyms and fitness centers":    "17f56080-837c-5ebb-a828-93fcb891bb8f",
	"Education":                   "51ed2111-26c8-5a0f-bffb-42aa00b5a0ce",
	"Bookstore":                   "51ed2111-26c8-5a0f-bffb-42aa00b5a0ce",
	"Office supplies":             "51ed2111-26c8-5a0f-bffb-42aa00b5a0ce",
	"Leisure":                     "659c118e-42ad-5427-898e-cea0153525ed",
	"Cinema/theater/concerts":     "659c118e-42ad-5427-898e-cea0153525ed",
	"Tickets":                     "659c118e-42ad-5427-898e-cea0153525ed",
	"Gambling":                    "659c118e-42ad-5427-898e-cea0153525ed",
	"Accomodation":                "a5318bcc-de1c-52c3-889b-2142e9c02408",
	"Airport and airlines":        "a5318bcc-de1c-52c3-889b-2142e9c02408",
	"Shopping":                    "11d50e25-577d-5780-bbae-35a6b14c7d01",
	"Online shopping":             "11d50e25-577d-5780-bbae-35a6b14c7d01",
	"Clothing":                    "11d50e25-577d-5780-bbae-35a6b14c7d01",
	"Electronics":                 "11d50e25-577d-5780-bbae-35a6b14c7d01",
	"Houseware":                   "11d50e25-577d-5780-bbae-35a6b14c7d01",
	"Digital services":            "1dddd1b3-1a1e-5c05-9a75-6359c68ce466",
	"Music streaming":             "1dddd1b3-1a1e-5c05-9a75-6359c68ce466",
	"Services":                    "aec4034c-f5a2-59a0-a1f9-1d737e4f1f3f",
	"Tax on financial operations": "2231c10d-ff72-59af-9513-72516f9f452e",
	"Cashback":                    "be4336bf-2090-58cd-babe-c7755e035619",
	SamePersonTransferLabel:       "533d9187-99b6-542b-a2f3-6eb9cbb299ce", // Transferência entre Contas Próprias, kind=transfer
}
