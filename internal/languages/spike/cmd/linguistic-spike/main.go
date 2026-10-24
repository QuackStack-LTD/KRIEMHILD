// linguistic-spike runs the isolated Phase 0 experiment. It has no network,
// database, model API, live World or automatic lexicon mutation integration.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"kriemhild/internal/languages/spike"
	"os"
	"strings"
)

type featureFlags map[string]string

func (f featureFlags) String() string { return string(spike.Canonical(f)) }
func (f featureFlags) Set(value string) error {
	k, v, ok := strings.Cut(value, "=")
	if !ok || k == "" || v == "" {
		return fmt.Errorf("features use key=value")
	}
	if _, duplicate := f[k]; duplicate {
		return fmt.Errorf("feature %s was supplied twice", k)
	}
	f[k] = v
	return nil
}
func main() {
	definition := flag.String("definition", "", "Path to an explicitly authored spike definition JSON")
	operation := flag.String("operation", "compile", "compile, generate, inflect, analyze, analyze-written, validate")
	lexeme := flag.String("lexeme", "", "Lexeme ID for inflection")
	template := flag.String("template", "", "Authored template ID for generation")
	segments := flag.String("segments", "", "Space-separated segment IDs for phonological input")
	text := flag.String("text", "", "Written input for written analysis")
	policy := flag.String("policy", "all", "all, seeded or explicit")
	seed := flag.String("seed", "", "Recorded author seed for seeded selection")
	limit := flag.Int("limit", 256, "Candidate display limit, 1–256")
	output := flag.String("output", "", "Optional JSON artifact path; default is stdout")
	features := featureFlags{}
	flag.Var(features, "feature", "Grammatical feature key=value; repeat for distinct keys")
	flag.Parse()
	fatal := func(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
	if *definition == "" {
		fatal(fmt.Errorf("choose an explicit -definition file"))
	}
	file, err := os.Open(*definition)
	if err != nil {
		fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	_ = file.Close()
	if err != nil {
		fatal(err)
	}
	d, err := spike.Decode(raw)
	if err != nil {
		fatal(err)
	}
	c, report := spike.Compile(context.Background(), d, spike.TokenTrieBackend{})
	var value any = report
	failed := report.Status == "invalid"
	if *operation != "compile" && !failed {
		// The CLI explicitly compiles this file now, then requests that EXACT
		// revision/engine. There is no persisted "latest" revision substitution.
		r := spike.Request{Revision: report.Revision, Engine: spike.EngineVersion, Operation: *operation, Lexeme: *lexeme, Template: *template, Segments: strings.Fields(*segments), Text: *text, Features: spike.Features(features), Policy: *policy, Seed: *seed, Limit: *limit}
		result := c.Execute(context.Background(), r)
		value = result
		failed = result.Status != "success" && result.Status != "ambiguous" && result.Status != "choice_required" && result.Status != "candidate_limit"
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatal(err)
	}
	data = append(data, '\n')
	if *output != "" {
		if err = os.WriteFile(*output, data, 0644); err != nil {
			fatal(err)
		}
	} else {
		_, _ = os.Stdout.Write(data)
	}
	if failed {
		os.Exit(1)
	}
}
