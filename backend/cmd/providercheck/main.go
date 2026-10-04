// Command providercheck makes an explicit provider diagnostic call.
package main

import (
	"os"
	"table-for-you/backend/internal/llm"
)

func main() { llm.Diagnose(os.Args) }
