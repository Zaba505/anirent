package cmd

import (
	"context"
	"os"
	"os/signal"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}
