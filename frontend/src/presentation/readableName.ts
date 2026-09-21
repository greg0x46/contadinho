// Institutions send shouting legal names — "CDB - NU FINANCEIRA S.A. -
// SOCIEDADE DE CREDITO, FINANCIAMENTO E INVESTIMENTO" — that bury what a
// person actually recognises. This turns them into something scannable for
// lists and pickers without touching the stored value; anywhere the raw text
// matters (detail, tooltip, search) keeps using the original.

// Whole segments (between " - ") that only describe the legal form.
const LEGAL_FORM_SEGMENT =
  /^(sociedade\s+(de\s+)?(credito|cr[eé]dito)[\s,]+financiamento\s+e\s+investimento|sociedade\s+an[oô]nima|(distribuidora|corretora)\s+de\s+t[ií]tulos\s+e\s+valores\s+mobili[aá]rios|corretora\s+de\s+valores|banco\s+m[uú]ltiplo|institui[cç][aã]o\s+de\s+pagamento|cr[eé]dito[\s,]+financiamento\s+e\s+investimento)\.?$/i;

// Suffixes that ride along at the end of a name.
const LEGAL_SUFFIX = /(?:[\s,]+(?:s\.?\s?a\.?|s\/a|ltda\.?|eireli|epp|me|dtvm|ctvm|cfi))+\s*$/i;

// Tokens kept in caps even inside a shouting name.
const ACRONYMS = new Set([
  "XP", "BTG", "BB", "CDB", "LCI", "LCA", "CRI", "CRA", "CDCA", "RDB", "LC", "LF", "COE", "ETF", "FII",
  "PGBL", "VGBL", "DI", "CDI", "IPCA", "SELIC", "RF", "FGC", "PIX", "TED", "DOC",
]);
const CONNECTIVES = new Set(["de", "da", "do", "das", "dos", "e", "em", "para", "por", "com", "a", "o"]);

function isShouting(segment: string): boolean {
  return !/\p{Ll}/u.test(segment) && /\p{Lu}{4,}/u.test(segment);
}

function titleCaseWord(word: string, first: boolean): string {
  if (/\d/.test(word) || ACRONYMS.has(word)) return word;
  const lower = word.toLocaleLowerCase("pt-BR");
  if (!first && CONNECTIVES.has(lower)) return lower;
  return lower.replace(/^\p{L}/u, (letter) =>
    letter.toLocaleUpperCase("pt-BR"),
  );
}

function cleanSegment(segment: string): string {
  const trimmed = segment.replace(/\s+/g, " ").trim();
  if (trimmed === "" || LEGAL_FORM_SEGMENT.test(trimmed)) return "";
  const bare = trimmed.replace(LEGAL_SUFFIX, "").replace(/[\s,]+$/, "");
  if (bare === "") return "";
  if (!isShouting(bare)) return bare;
  return bare
    .split(" ")
    .map((word, index) => titleCaseWord(word, index === 0))
    .join(" ");
}

/**
 * A recognisable version of a provider label: legal-form boilerplate dropped,
 * ALL-CAPS names title-cased, acronyms and tickers untouched. Returns the
 * input (whitespace-collapsed) when there is nothing to clean.
 */
export function readableName(raw: string): string {
  const segments = raw
    .split(/\s+-\s+/)
    .map(cleanSegment)
    .filter(Boolean);
  const result = segments.join(" - ");
  return result === "" ? raw.replace(/\s+/g, " ").trim() : result;
}
