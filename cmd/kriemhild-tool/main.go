// Companion CLI for read-only legacy backup and native project validation.
package main

import (
	"flag"
	"fmt"
	"kriemhild/internal/project"
	"os"
)

func main() {
	source := flag.String("source", "", "complete project folder, including project.head.json")
	output := flag.String("output", "", "new native archive file; existing files are never overwritten")
	flag.Parse()
	if *source == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "Usage: kriemhild-tool -source <project folder> -output <new archive.zip>")
		os.Exit(2)
	}
	f, e := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	e = project.WriteArchiveDirectory(*source, f)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		os.Remove(*output) // Only the file exclusively created by this invocation.
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println("Native backup created without changing the source project.")
}
