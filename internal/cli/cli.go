package cli

import (
	"errors"
	"fmt"
)

func Run(args []string) error {
	if len(args) == 0 {
		return errors.New("a subcommand is required")
	}

	switch args[0] {
	case "add":
		return fmt.Errorf("not implemented yet")
	case "remove":
		return fmt.Errorf("not implemented yet")
	case "list":
		return fmt.Errorf("not implemented yet")
	case "help":
		printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func printHelp() {
	fmt.Println(`Usage: vigi l <command> [args]

Commands:
  add <name> --type <node|static> ...   Register a new app
  remove <name>                         Remove a registered app
  list                                  List all registered apps
  help                                  Show this help`)
}
