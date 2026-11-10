package cleanup

import (
	"strings"
	"unicode/utf8"
)

// dictionaryPromptChars is what the dictionary may use of the prompt, so a long dictionary cannot crowd out the
// transcript. Screen words have no budget: the prompt carries all of them.
const dictionaryPromptChars = 4000

const cleanInstruction = "Clean this speech-to-text transcript. Correct punctuation, capitalization, grammar, and clear transcription errors without changing its meaning, tone, or wording."

// Prompt is the user message for one transcript: the instruction, the terms the transcript may use, and the transcript.
func Prompt(transcript string, terms Terms) string {
	var prompt strings.Builder
	prompt.WriteString(cleanInstruction)
	if dictionary := termList(terms.Dictionary, dictionaryPromptChars); dictionary != "" {
		prompt.WriteString("\nPreferred terms and spellings (use only when supported by the dictated context):\n" + dictionary + "\n")
	}
	if screen := strings.TrimSpace(strings.Join(terms.Screen, "\n")); screen != "" {
		prompt.WriteString("\nScreen terms (words seen on screen; use only when supported by the dictated context):\n" + screen + "\n")
	}
	prompt.WriteString("\nTranscript:\n" + transcript)
	return prompt.String()
}

// termList joins terms one per line and keeps at most limit characters of them.
func termList(terms []string, limit int) string {
	text := strings.TrimSpace(strings.Join(terms, "\n"))
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit])
}
