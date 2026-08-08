package main

import (
	"github.com/gobravedev/opencode/cmd"
	"github.com/gobravedev/opencode/internal/logging"
)

func main() {
	defer logging.RecoverPanic("main", func() {
		logging.ErrorPersist("Application terminated due to unhandled panic")
	})

	cmd.Execute()
}
