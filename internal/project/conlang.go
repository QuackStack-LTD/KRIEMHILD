package project

import (
	"fmt"
	"strings"
)

type soundRule struct {
	from        string
	to          []string
	left, right string
}

func soundClasses(raw string) (map[string]map[string]bool, error) {
	classes := map[string]map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) != 2 || len(strings.Fields(parts[0])) != 1 {
			return nil, fmt.Errorf("sound classes use Name = token token")
		}
		name := strings.TrimSpace(parts[0])
		tokens := strings.Fields(parts[1])
		if len(tokens) == 0 || len(tokens) > 300 || len(classes) > 100 {
			return nil, fmt.Errorf("sound class exceeds token budget")
		}
		classes[name] = map[string]bool{}
		for _, token := range tokens {
			classes[name][token] = true
		}
	}
	return classes, nil
}
func parseSoundRules(raw string) ([]soundRule, error) {
	out := []soundRule{}
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		halves := strings.Split(line, "/")
		if len(halves) > 2 {
			return nil, fmt.Errorf("use token > replacement / left _ right")
		}
		parts := strings.Split(halves[0], ">")
		if len(parts) != 2 || len(strings.Fields(parts[0])) != 1 || len(strings.Fields(parts[1])) > 8 {
			return nil, fmt.Errorf("sound rules need one source token and up to eight replacement tokens")
		}
		r := soundRule{from: strings.TrimSpace(parts[0]), to: strings.Fields(parts[1])}
		if len(halves) == 2 {
			context := strings.Split(halves[1], "_")
			if len(context) != 2 || len(strings.Fields(context[0])) > 1 || len(strings.Fields(context[1])) > 1 {
				return nil, fmt.Errorf("each sound-rule context is one token, class, # boundary, or empty wildcard")
			}
			r.left = strings.TrimSpace(context[0])
			r.right = strings.TrimSpace(context[1])
		}
		out = append(out, r)
	}
	if len(out) == 0 || len(out) > 100 {
		return nil, fmt.Errorf("use 1–100 ordered sound rules")
	}
	return out, nil
}
func soundMatch(pattern string, tokens []string, index int, classes map[string]map[string]bool) bool {
	if pattern == "" {
		return true
	}
	if index < 0 || index >= len(tokens) {
		return pattern == "#"
	}
	if pattern == "#" {
		return false
	}
	if class, ok := classes[pattern]; ok {
		return class[tokens[index]]
	}
	return pattern == tokens[index]
}
func applySoundRules(tokens []string, rules []soundRule, classes map[string]map[string]bool) ([]string, error) {
	for _, rule := range rules {
		next := []string{}
		for i, token := range tokens {
			if soundMatch(rule.from, tokens, i, classes) && soundMatch(rule.left, tokens, i-1, classes) && soundMatch(rule.right, tokens, i+1, classes) {
				next = append(next, rule.to...)
			} else {
				next = append(next, token)
			}
		}
		tokens = next
		if len(tokens) > 10000 {
			return nil, fmt.Errorf("sound changes exceed output budget")
		}
	}
	return tokens, nil
}
func conlangExperiment(c ExperimentRequest, st State, selected []Record, out *ExperimentResult) error {
	classes, e := soundClasses(c.Values["classes"])
	if e != nil {
		return e
	}
	switch c.Operation {
	case "phonotactics":
		patterns := [][]string{}
		for _, p := range strings.Split(c.Values["patterns"], "\n") {
			tokens := strings.Fields(p)
			if len(tokens) > 0 {
				patterns = append(patterns, tokens)
			}
		}
		if len(patterns) == 0 || len(patterns) > 100 {
			return fmt.Errorf("provide 1–100 token patterns, such as C V C")
		}
		for _, r := range selected {
			if r.Properties["_domain"] != "lexeme" {
				continue
			}
			raw, _ := r.Properties["phonemes"].(string)
			tokens := strings.Fields(raw)
			match := "No matching pattern"
			for _, p := range patterns {
				if len(p) != len(tokens) {
					continue
				}
				ok := true
				for i, t := range p {
					if !soundMatch(t, tokens, i, classes) {
						ok = false
						break
					}
				}
				if ok {
					match = strings.Join(p, " ")
					break
				}
			}
			out.Rows = append(out.Rows, map[string]string{"entry": r.Name, "phonemes": raw, "matching pattern": match})
		}
		out.Warnings = append(out.Warnings, "Checks complete token sequences against explicitly listed patterns. Exceptions and missing phonemes remain the author's decision.")
	case "inflection":
		type formRule struct{ label, prefix, suffix string }
		rules := []formRule{}
		seen := map[string]bool{}
		for _, line := range strings.Split(c.Values["affixes"], "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			p := strings.Split(line, "|")
			if len(p) != 3 || !nameOK(strings.TrimSpace(p[0])) || seen[strings.TrimSpace(p[0])] {
				return fmt.Errorf("affixes use unique label | prefix | suffix, one per line")
			}
			label := strings.TrimSpace(p[0])
			seen[label] = true
			rules = append(rules, formRule{label, strings.TrimSpace(p[1]), strings.TrimSpace(p[2])})
		}
		if len(rules) == 0 || len(rules) > 100 {
			return fmt.Errorf("use 1–100 inflection forms")
		}
		for _, r := range selected {
			if r.Properties["_domain"] != "lexeme" {
				continue
			}
			stem, _ := r.Properties["form"].(string)
			if stem == "" {
				out.Warnings = append(out.Warnings, r.Name+": no written form to inflect")
				continue
			}
			forms := map[string]string{}
			for _, rule := range rules {
				form := rule.prefix + stem + rule.suffix
				if len(form) > 10000 {
					return fmt.Errorf("inflection exceeds length budget")
				}
				forms[rule.label] = form
				out.Rows = append(out.Rows, map[string]string{"entry": r.Name, "grammatical form": rule.label, "result": form})
			}
			r = cloneProperties(r)
			r.Properties["inflections"] = forms
			r.Properties["inflectionRules"] = c.Values["affixes"]
			r.Properties["derivationSnapshot"] = st.Age.Snapshot
			out.Proposals = append(out.Proposals, r)
		}
		out.Warnings = append(out.Warnings, "Concatenative affixes only. Accepting replaces this entry's stored inflection table; irregular forms and agreement are not inferred.")
	}
	return nil
}
