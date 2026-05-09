package main

import (
	"context"
	"fmt"
	"os"

	"github.com/revrost/mpal-cli/internal/mcpserver"
)

func main() {
	if err := mcpserver.RunStdio(context.Background(), mcpserver.DefaultConfig()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "mpal-mcp failed: %v\n", err)
		os.Exit(1)
	}
}
