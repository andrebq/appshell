package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/andrebq/appshell/gui"
	"github.com/andrebq/appshell/shell"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	sh := shell.New()
	sh.EnableJSONRPCClient()

	gui.Run(ctx, sh)
}
