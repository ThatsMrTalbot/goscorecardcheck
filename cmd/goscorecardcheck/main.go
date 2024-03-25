package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/thatsmrtalbot/goscorecardcheck/internal/command"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd := command.NewScoreCardCheckCommand()
	if err := cmd.ExecuteContext(ctx); err != nil {
		log.Fatal(err)
	}
}
