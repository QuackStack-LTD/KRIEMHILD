// Command languages replays the same bounded product engine without a database,
// network service or UI. Inputs are authored definition/script/request JSON.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"kriemhild/internal/languages"
	"kriemhild/internal/languages/spike"
	"os"
	"time"
)

type paths []string

func (p *paths) String() string     { return fmt.Sprint([]string(*p)) }
func (p *paths) Set(s string) error { *p = append(*p, s); return nil }
func read(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, languages.MaxDefinitionBytes+1))
	if len(raw) > languages.MaxDefinitionBytes {
		return nil, fmt.Errorf("input exceeds 2 MiB")
	}
	return raw, err
}
func run() error {
	definitionPath := flag.String("definition", "", "Authored language definition JSON")
	requestPath := flag.String("request", "", "Exact versioned execution request JSON")
	sourcePath := flag.String("source", "", "Pinned source definition for historical preview")
	operation := flag.String("operation", "compile", "compile, run, history-preview, script-check, script-compile, script-render, svg-import")
	corpusPath := flag.String("corpus", "", "Strict sequence-corpus/1 JSON for corpus-validate or corpus-run")
	var corpusDefinitions paths
	flag.Var(&corpusDefinitions, "corpus-definition", "Exact immutable language source; repeat for retained revisions")
	glyphID := flag.String("glyph-id", "imported-glyph", "Stable glyph identity for SVG normalization")
	glyphName := flag.String("glyph-name", "Imported artwork", "Authored glyph name for SVG normalization")
	var scriptPaths paths
	flag.Var(&scriptPaths, "script", "Pinned script JSON; repeat for each dependency")
	flag.Parse()
	if len(scriptPaths) > 8 {
		return fmt.Errorf("at most eight pinned script files are accepted")
	}
	scripts := map[languages.ScriptRef]languages.ScriptDefinition{}
	for _, path := range scriptPaths {
		raw, err := read(path)
		if err != nil {
			return err
		}
		script, err := languages.DecodeScript(raw)
		if err != nil {
			return err
		}
		scripts[languages.ScriptRef{ID: script.ScriptID, Revision: languages.Hash(script)}] = script
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if *operation == "corpus-validate" || *operation == "corpus-run" {
		if *definitionPath != "" {
			corpusDefinitions = append(corpusDefinitions, *definitionPath)
		}
		if len(corpusDefinitions) > 8 {
			return &corpusExit{2, "at most eight immutable language files"}
		}
		bodies := map[string][]byte{}
		for ref, s := range scripts {
			bodies["script:"+ref.ID+":"+ref.Revision] = languages.Canonical(s)
		}
		for _, path := range corpusDefinitions {
			raw, err := read(path)
			if err != nil {
				return &corpusExit{2, err.Error()}
			}
			d, err := languages.Decode(raw)
			if err != nil {
				return &corpusExit{2, err.Error()}
			}
			bodies["language:"+d.LanguageID+":"+languages.Hash(d)] = languages.Canonical(d)
		}
		raw, err := read(*corpusPath)
		if err != nil {
			return &corpusExit{2, err.Error()}
		}
		corpus, err := languages.DecodeSequenceCorpus(raw)
		if err != nil {
			return &corpusExit{2, err.Error()}
		}
		resolve := func(_ context.Context, kind, id, revision string) ([]byte, error) {
			raw, ok := bodies[kind+":"+id+":"+revision]
			if !ok {
				return nil, fmt.Errorf("missing exact immutable pin")
			}
			return raw, nil
		}
		report, err := languages.RunSequenceCorpus(ctx, corpus, resolve, *operation == "corpus-validate")
		if err != nil {
			if encodeErr := encoder.Encode(report); encodeErr != nil {
				return encodeErr
			}
			if report.Status == "cancelled" || report.Status == "limit_reached" {
				return &corpusExit{3, err.Error()}
			}
			return &corpusExit{2, err.Error()}
		}
		if err = encoder.Encode(report); err != nil {
			return err
		}
		fmt.Fprint(os.Stderr, languages.CorpusSummary(report))
		if report.Status == "cancelled" || report.Status == "limit_reached" {
			return &corpusExit{3, ""}
		}
		if report.Status != "success" {
			return &corpusExit{1, ""}
		}
		return nil
	}
	if *operation == "svg-import" {
		raw, err := read(*sourcePath)
		if err != nil {
			return err
		}
		result, err := languages.ImportGlyphSVG(ctx, string(raw), *glyphID, *glyphName)
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	}
	if *operation == "script-compile" || *operation == "script-render" {
		if len(scripts) != 1 {
			return fmt.Errorf("script execution requires exactly one pinned script")
		}
		for _, script := range scripts {
			if *operation == "script-compile" {
				return encoder.Encode(languages.CompileScript(ctx, script))
			}
			raw, err := read(*requestPath)
			if err != nil {
				return err
			}
			var q languages.ScriptRenderRequest
			if err = spike.DecodeJSON(raw, &q); err != nil {
				return err
			}
			return encoder.Encode(languages.RenderScript(ctx, script, q))
		}
	}
	if *operation == "script-check" {
		if len(scripts) != 1 {
			return fmt.Errorf("script-check requires exactly one script")
		}
		for ref := range scripts {
			return encoder.Encode(ref)
		}
	}
	raw, err := read(*definitionPath)
	if err != nil {
		return err
	}
	definition, err := languages.Decode(raw)
	if err != nil {
		return err
	}
	if *operation == "history-preview" {
		raw, err := read(*sourcePath)
		if err != nil {
			return err
		}
		source, err := languages.Decode(raw)
		if err != nil {
			return err
		}
		return encoder.Encode(languages.PreviewEvolution(ctx, source, definition))
	}
	artifact := languages.Compile(ctx, definition, scripts)
	if *operation == "compile" {
		return encoder.Encode(artifact.Report())
	}
	if *operation != "run" {
		return fmt.Errorf("unsupported CLI operation")
	}
	raw, err = read(*requestPath)
	if err != nil {
		return err
	}
	var request languages.Request
	if err = spike.DecodeJSON(raw, &request); err != nil {
		return err
	}
	return encoder.Encode(artifact.Execute(ctx, request))
}

type corpusExit struct {
	code    int
	message string
}

func (e *corpusExit) Error() string { return e.message }
func main() {
	if err := run(); err != nil {
		if err.Error() != "" {
			fmt.Fprintln(os.Stderr, err)
		}
		var corpusError *corpusExit
		if errors.As(err, &corpusError) {
			os.Exit(corpusError.code)
		}
		os.Exit(1)
	}
}
