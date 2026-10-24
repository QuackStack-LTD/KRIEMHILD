package languages

// Glosses expose all authored senses and grammatical feature values. They are
// annotations, never a fabricated natural-language translation.
func (a *Artifact) gloss(sentence *Sentence, c Construction) {
	for _, part := range c.Order {
		if part.Role == "" {
			continue
		}
		form := sentence.Roles[part.Role]
		word := GlossWord{Role: part.Role, Lexeme: form.Lexeme, Surface: surface(form), Features: Clone(form.Features), Meanings: []string{}}
		for _, lex := range a.definition.Lexicon {
			if lex.ID == form.Lexeme {
				for _, sense := range lex.Senses {
					word.Meanings = append(word.Meanings, sense.Meaning)
				}
				break
			}
		}
		sentence.Gloss = append(sentence.Gloss, word)
	}
}
