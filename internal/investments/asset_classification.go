package investments

import "strings"

// AssetClass describes financial exposure independently from custody and the
// market used to obtain prices. An ETF may hold bonds, shares or crypto.
type AssetClass string

const (
	AssetClassFixedIncome    AssetClass = "fixed_income"
	AssetClassVariableIncome AssetClass = "variable_income"
	AssetClassMultimarket    AssetClass = "multimarket"
	AssetClassCurrency       AssetClass = "currency"
	AssetClassCrypto         AssetClass = "crypto"
	AssetClassOther          AssetClass = "other"
)

type AssetTypeDefinition struct {
	Name string `json:"name"`
	// QuoteMarket is routing information, not a financial classification.
	QuoteMarket *string `json:"quote_market"`
}

type AssetClassDefinition struct {
	Class AssetClass            `json:"asset_class"`
	Label string                `json:"label"`
	Types []AssetTypeDefinition `json:"types"`
}

// AssetClassification is shared by validation and the asset form. Unknown
// imported types remain editable; known types must match the chosen class.
func AssetClassification() []AssetClassDefinition {
	types := func(market string, names ...string) []AssetTypeDefinition {
		result := make([]AssetTypeDefinition, 0, len(names))
		for _, name := range names {
			var quoteMarket *string
			if market != "" {
				value := market
				quoteMarket = &value
			}
			result = append(result, AssetTypeDefinition{Name: name, QuoteMarket: quoteMarket})
		}
		return result
	}
	return []AssetClassDefinition{
		{AssetClassFixedIncome, "Renda fixa", append(types("", "Título público", "CDB", "RDB", "LCI", "LCA", "Debênture", "CRI", "CRA", "Poupança", "Fundo de renda fixa", "PGBL", "VGBL"), types("b3", "ETF de renda fixa")...)},
		{AssetClassVariableIncome, "Renda variável", append(types("b3", "Ação", "Unit", "BDR de ação", "FII", "Fiagro", "ETF de ações"), types("", "Fundo de ações", "PGBL", "VGBL")...)},
		{AssetClassMultimarket, "Multimercado", types("", "Fundo multimercado", "PGBL", "VGBL")},
		{AssetClassCurrency, "Cambial", types("", "Fundo cambial", "Moeda estrangeira")},
		{AssetClassCrypto, "Criptoativos", append(types("crypto", "Criptomoeda", "Stablecoin", "Token"), types("b3", "ETF de criptoativos")...)},
		{AssetClassOther, "Outros", append(types("", "COE", "Derivativo", "Ouro"), types("b3", "ETF de commodities")...)},
	}
}

func IsAssetClass(class AssetClass) bool {
	for _, definition := range AssetClassification() {
		if definition.Class == class {
			return true
		}
	}
	return false
}

// InferAssetClass recognizes old manual labels and provider labels without
// guessing a generic fund's or ETF's underlying exposure.
func InferAssetClass(assetType string) AssetClass {
	kind := strings.ToLower(strings.TrimSpace(assetType))
	var found AssetClass
	for _, definition := range AssetClassification() {
		for _, candidate := range definition.Types {
			if strings.ToLower(candidate.Name) == kind {
				if found != "" && found != definition.Class {
					return AssetClassOther
				}
				found = definition.Class
			}
		}
	}
	if found != "" {
		return found
	}
	switch kind {
	case "fixed_income", "renda fixa", "tesouro", "tesouro direto", "government_bond", "corporate_bond", "debenture", "debentures":
		return AssetClassFixedIncome
	case "equity", "stock", "stocks", "ações", "acao", "real_estate_fund", "fundo imobiliário", "bdr", "renda variável", "renda variavel":
		return AssetClassVariableIncome
	case "multimercado", "multimarket":
		return AssetClassMultimarket
	case "cambial", "currency":
		return AssetClassCurrency
	case "criptoativo", "criptoativos", "crypto", "cryptocurrency":
		return AssetClassCrypto
	default:
		return AssetClassOther
	}
}

func assetTypeMatchesClass(class AssetClass, assetType string) bool {
	kind := strings.ToLower(strings.TrimSpace(assetType))
	known := false
	for _, definition := range AssetClassification() {
		for _, candidate := range definition.Types {
			if strings.ToLower(candidate.Name) == kind {
				known = true
				if definition.Class == class {
					return true
				}
			}
		}
	}
	// Custom and imported descriptions are preserved rather than discarded.
	if class == AssetClassOther && InferAssetClass(assetType) == AssetClassOther {
		return true
	}
	if known {
		return false
	}
	inferred := InferAssetClass(assetType)
	return inferred == AssetClassOther || class == inferred
}
