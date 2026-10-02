// Offline account recovery for a library whose hosted server is stopped.
package main

import (
	"flag"
	"fmt"
	"kriemhild/internal/httpapi"
	"log"
	"os"
)

func main() {
	library := flag.String("data", "", "existing hosted library directory (required)")
	user := flag.String("user", "admin", "existing account whose password is reset")
	flag.Parse()
	if *library == "" {
		log.Fatal("-data is required; stop the hosted server before using this tool")
	}
	password := os.Getenv("KRIEMHILD_RESET_PASSWORD")
	if password == "" {
		log.Fatal("set KRIEMHILD_RESET_PASSWORD to the new password; it is never accepted as a command-line argument")
	}
	if e := httpapi.ResetHostedPassword(*library, *user, password); e != nil {
		log.Fatal(e)
	}
	fmt.Printf("Password reset for %s. Accounts, memberships and world content were preserved. Start the hosted server to sign in.\n", *user)
}
