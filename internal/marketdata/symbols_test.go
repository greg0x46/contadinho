package marketdata

import (
	"errors"
	"strings"
	"testing"
)

func TestParseMarket(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  Market
		ok    bool
	}{
		{"b3", MarketB3, true},
		{" crypto ", MarketCrypto, true},
		{"B3", "", false},
		{"brapi", "", false},
		{"", "", false},
	} {
		got, ok := ParseMarket(tc.input)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseMarket(%q) = %q, %v; want %q, %v", tc.input, got, ok, tc.want, tc.ok)
		}
	}
	if markets := Markets(); len(markets) != 2 || markets[0] != MarketB3 || markets[1] != MarketCrypto {
		t.Errorf("Markets() = %v", markets)
	}
}

func TestParseInstrumentB3AcceptsEverySpellingOfATicker(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"PETR4", "PETR4"},
		{"petr4", "PETR4"},
		{"  petr4  ", "PETR4"},
		{"PETR4.SA", "PETR4"},
		{"petr4.sa", "PETR4"},
		{"BVMF:PETR4", "PETR4"},
		{"B3:PETR4", "PETR4"},
		{"BOVESPA:petr4", "PETR4"},
		{"BOVA11", "BOVA11"},
		{"B3SA3", "B3SA3"},
		{"PETR4F", "PETR4F"},
		{"AAPL34", "AAPL34"},
		{"HGLG11", "HGLG11"},
	} {
		got, err := ParseInstrument(MarketB3, tc.input)
		if err != nil {
			t.Errorf("ParseInstrument(b3, %q): %v", tc.input, err)
			continue
		}
		if got != (Instrument{Market: MarketB3, Symbol: tc.want}) {
			t.Errorf("ParseInstrument(b3, %q) = %+v, want %s", tc.input, got, tc.want)
		}
	}
}

func TestParseInstrumentB3RefusesWhatIsNotAB3Ticker(t *testing.T) {
	for _, input := range []string{"", "   ", "PETR", "PETR444", "BTC", "BTC-USD", "bitcoin", "PE TR4", "^BVSP", "4PETR"} {
		_, err := ParseInstrument(MarketB3, input)
		var symbolErr *SymbolError
		if !errors.As(err, &symbolErr) {
			t.Errorf("ParseInstrument(b3, %q) error = %v, want a *SymbolError", input, err)
			continue
		}
		if symbolErr.Market != MarketB3 || symbolErr.Input != input || !strings.Contains(symbolErr.Reason, "código da B3") {
			t.Errorf("ParseInstrument(b3, %q) = %+v", input, symbolErr)
		}
	}
}

func TestParseInstrumentCryptoIsTheTickerWithoutItsPair(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"BTC", "BTC"},
		{"btc", "BTC"},
		{" eth ", "ETH"},
		{"BTC-USD", "BTC"},
		{"btc-brl", "BTC"},
		{"BTC/BRL", "BTC"},
		{"eth/brl", "ETH"},
		{"SOL-USDT", "SOL"},
		{"USDT", "USDT"},
		{"1INCH", "1INCH"},
		{"bitcoin", "BITCOIN"},
	} {
		got, err := ParseInstrument(MarketCrypto, tc.input)
		if err != nil {
			t.Errorf("ParseInstrument(crypto, %q): %v", tc.input, err)
			continue
		}
		if got != (Instrument{Market: MarketCrypto, Symbol: tc.want}) {
			t.Errorf("ParseInstrument(crypto, %q) = %+v, want %s", tc.input, got, tc.want)
		}
	}
}

func TestParseInstrumentCryptoRefusesWhatIsNotATicker(t *testing.T) {
	for _, input := range []string{"", "  ", "bit coin", "B", "BTC-XYZ", "BTC.SA", "-USD", "AVERYVERYLONGTICKER"} {
		_, err := ParseInstrument(MarketCrypto, input)
		var symbolErr *SymbolError
		if !errors.As(err, &symbolErr) {
			t.Errorf("ParseInstrument(crypto, %q) error = %v, want a *SymbolError", input, err)
			continue
		}
		if !strings.Contains(symbolErr.Reason, "criptomoeda") {
			t.Errorf("reason = %q", symbolErr.Reason)
		}
	}
}

func TestParseInstrumentUnknownMarket(t *testing.T) {
	_, err := ParseInstrument("nasdaq", "AAPL")
	var symbolErr *SymbolError
	if !errors.As(err, &symbolErr) || symbolErr.Reason != "mercado de cotação desconhecido" {
		t.Fatalf("error = %v, want an unknown market SymbolError", err)
	}
}

func TestProviderSymbols(t *testing.T) {
	petr4 := Instrument{Market: MarketB3, Symbol: "PETR4"}
	btc := Instrument{Market: MarketCrypto, Symbol: "BTC"}
	if got := yahooSymbol(petr4); got != "PETR4.SA" {
		t.Errorf("yahooSymbol(PETR4) = %q", got)
	}
	if got := yahooSymbol(btc); got != "BTC-USD" {
		t.Errorf("yahooSymbol(BTC) = %q", got)
	}
	if got := brapiSymbol(petr4); got != "PETR4" {
		t.Errorf("brapiSymbol(PETR4) = %q", got)
	}
}
