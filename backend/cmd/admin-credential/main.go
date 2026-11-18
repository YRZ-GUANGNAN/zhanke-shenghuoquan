package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/term"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/database"
	"campus-circle/backend/internal/module/adminauth"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 || (args[0] != "bootstrap" && args[0] != "reset") {
		return errors.New("usage: admin-credential <bootstrap|reset> --username <name> [--role superadmin] [--display-name <name>]")
	}
	operation := args[0]
	flags := flag.NewFlagSet(operation, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	usernameValue := flags.String("username", "", "administrator login username (3-64 letters, digits, dot or underscore)")
	roleValue := flags.String("role", "superadmin", "administrator role: admin or superadmin")
	displayNameValue := flags.String("display-name", "", "administrator display name (defaults to username)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	username := strings.TrimSpace(*usernameValue)
	if flags.NArg() != 0 || username == "" {
		return errors.New("--username is required and no password argument is accepted")
	}
	role := adminauth.Role(strings.TrimSpace(*roleValue))
	if !role.Valid() {
		return errors.New("--role must be admin or superadmin")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("password input requires an interactive TTY")
	}
	fmt.Fprint(os.Stderr, "管理员新密码（输入不会显示）: ")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return errors.New("read password from TTY failed")
	}
	fmt.Fprint(os.Stderr, "再次输入管理员新密码: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return errors.New("read password confirmation from TTY failed")
	}
	if string(first) != string(second) {
		return errors.New("password confirmation does not match")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	db, err := database.Open(cfg)
	if err != nil {
		return errors.New("open database failed")
	}
	service, err := adminauth.NewService(adminauth.NewRepository(db))
	if err != nil {
		return errors.New("initialize administrator account service failed")
	}
	eventType := "credential_bootstrap"
	if operation == "reset" {
		eventType = "credential_reset"
	}
	if err := service.Bootstrap(context.Background(), username, strings.TrimSpace(*displayNameValue), role, string(first), eventType); err != nil {
		return fmt.Errorf("%s administrator account: %w", operation, err)
	}
	fmt.Fprintln(os.Stdout, "管理员账号已安全更新，现有 Web 管理会话已按操作策略处理。")
	return nil
}
