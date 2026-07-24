package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"golang.org/x/term"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/database"
	"campus-circle/backend/internal/module/adminauth"
	"campus-circle/backend/internal/module/user"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 || (args[0] != "bootstrap" && args[0] != "reset") {
		return errors.New("usage: admin-credential <bootstrap|reset> --user-id <id>")
	}
	operation := args[0]
	flags := flag.NewFlagSet(operation, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	userIDValue := flags.String("user-id", "", "existing active superadmin user ID")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *userIDValue == "" {
		return errors.New("--user-id is required and no password argument is accepted")
	}
	parsed, err := strconv.ParseUint(*userIDValue, 10, 32)
	if err != nil || parsed == 0 {
		return errors.New("--user-id must be a positive integer")
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
	users := user.NewService(user.NewRepository(db), cfg.JWTSecret, cfg.WechatAppID, cfg.WechatAppSecret, cfg.AllowMockWechat)
	service, err := adminauth.NewService(adminauth.NewRepository(db), users)
	if err != nil {
		return errors.New("initialize credential service failed")
	}
	eventType := "credential_bootstrap"
	if operation == "reset" {
		eventType = "credential_reset"
	}
	if err := service.Bootstrap(context.Background(), uint(parsed), string(first), eventType); err != nil {
		return fmt.Errorf("%s administrator credential: %w", operation, err)
	}
	fmt.Fprintln(os.Stdout, "管理员凭据已安全更新，现有 Web 管理会话已按操作策略处理。")
	return nil
}
