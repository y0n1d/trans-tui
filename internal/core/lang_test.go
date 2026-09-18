package core

import "testing"

func TestIsChinese(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"pure chinese", "你好世界", true},
		{"chinese punctuation", "你好，世界！", true},
		{"chinese with digits", "第 1 章", true},
		{"english", "Hello world", false},
		{"french", "Bonjour le monde", false},
		{"japanese hiragana", "こんにちは", false},
		{"japanese kanji and kana", "日本語を勉強する", false},
		{"japanese katakana", "コンピュータ", false},
		{"korean hangul", "안녕하세요", false},
		{"korean with hanja", "한국어를 공부합니다", false},
		{"empty", "", false},
		{"digits only", "12345", false},
		{"symbols only", "!@#$%^&*()", false},
		{"whitespace only", "   \t\n", false},
		{"mixed dominant chinese", "这个 API 很好用", true},
		{"mixed dominant english", "Hello 世界", false},
		{"mixed english and chinese", "中文 mixed English 混排", false},
		{"mixed tie goes to chinese", "你好 ab", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsChinese(tc.text); got != tc.want {
				t.Errorf("IsChinese(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestResolveTargetLang(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		text       string
		want       string
	}{
		{"auto chinese to english", "auto", "你好世界", "en"},
		{"auto english to chinese", "auto", "Hello world", "zh"},
		{"auto japanese to chinese", "auto", "こんにちは", "zh"},
		{"auto korean to chinese", "auto", "안녕하세요", "zh"},
		{"auto chinese punctuation to english", "auto", "你好，世界！", "en"},
		{"auto french to chinese", "auto", "Bonjour", "zh"},
		{"auto empty to chinese", "auto", "", "zh"},
		{"auto digits to chinese", "auto", "12345", "zh"},
		{"auto symbols to chinese", "auto", "!@#$", "zh"},
		{"explicit zh preserved for chinese", "zh", "你好", "zh"},
		{"explicit en preserved for chinese", "en", "你好", "en"},
		{"explicit fr preserved", "fr", "你好", "fr"},
		{"empty configured preserved", "", "你好", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveTargetLang(tc.configured, tc.text); got != tc.want {
				t.Errorf("ResolveTargetLang(%q, %q) = %q, want %q",
					tc.configured, tc.text, got, tc.want)
			}
		})
	}
}
