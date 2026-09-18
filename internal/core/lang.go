package core

import "unicode"

// Language sentinels understood by the translation pipeline.
const (
	// LangAuto selects the target language from the input script: Chinese
	// input is translated to English, everything else to Chinese.
	LangAuto = "auto"
	// LangEnglish is the ISO 639-1 code for English.
	LangEnglish = "en"
	// LangChinese is the ISO 639-1 code for Chinese.
	LangChinese = "zh"
)

// ResolveTargetLang turns a configured target language into the concrete
// language handed to a provider.
//
// LangAuto chooses the target from the input text (Chinese -> English,
// anything else -> Chinese). Any other configured value is returned
// unchanged so existing explicit configurations keep working.
func ResolveTargetLang(configured, text string) string {
	if configured != LangAuto {
		return configured
	}
	if IsChinese(text) {
		return LangEnglish
	}
	return LangChinese
}

// IsChinese reports whether text should be treated as Chinese when choosing a
// translation target.
//
// The heuristic is dependency-free and deliberately conservative:
//
//   - Japanese (any hiragana/katakana) and Korean (any hangul) are never
//     treated as Chinese even though they use Han ideographs.
//   - Text with no Han ideograph is not Chinese.
//   - Otherwise the text is Chinese when Han ideographs make up at least
//     half of its letters. Digits, punctuation and whitespace are ignored;
//     letters from any other script count against the Han ratio.
//
// Mixed Chinese/English text therefore translates to English only when it is
// predominantly Chinese, and Japanese/Korean are never misdetected.
func IsChinese(text string) bool {
	var han, other int
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			han++
		case unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r):
			return false
		case unicode.Is(unicode.Hangul, r):
			return false
		case unicode.IsLetter(r):
			other++
		}
	}
	if han == 0 {
		return false
	}
	return han >= other
}
