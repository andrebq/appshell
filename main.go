package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/andrebq/appshell/gui"
	"github.com/andrebq/appshell/shell"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	sh := shell.New()
	sh.EnableJSONRPCClient()
	abs, err := filepath.Abs(".")
	if err != nil {
		panic(err)
	}
	sh.AllowImportFrom(abs)

	gui.Run(ctx, sh)
}
