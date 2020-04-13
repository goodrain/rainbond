package cmd

import (
	"fmt"
	"os"
)

func showWarn(m string) {
	fmt.Printf("warn: %s", m)
	os.Exit(0)
}
