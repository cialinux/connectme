package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/connectme/connectme/core/config"
	"github.com/connectme/connectme/core/database"
	"github.com/connectme/connectme/core/encryption"
	"github.com/connectme/connectme/modules/audit"
	"github.com/connectme/connectme/modules/authorization"
	"github.com/connectme/connectme/modules/identity"
	"log/slog"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "create-admin" {
		fmt.Fprintln(os.Stderr, "uso: connectmectl create-admin --email EMAIL --name NOME --password-stdin")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("create-admin", flag.ExitOnError)
	email := fs.String("email", "", "email")
	name := fs.String("name", "Administrator", "nome")
	stdin := fs.Bool("password-stdin", false, "ler senha da entrada padrão")
	_ = fs.Parse(os.Args[2:])
	if !*stdin || *email == "" {
		fmt.Fprintln(os.Stderr, "--email e --password-stdin são obrigatórios")
		os.Exit(2)
	}
	password, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(password) == 0 {
		fatal(err)
	}
	password = strings.TrimSpace(password)
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	pool, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()
	cipher, err := encryption.New(cfg.Security.MasterKey)
	if err != nil {
		fatal(err)
	}
	auditService := audit.NewService(pool, cfg.Security.AuditKey)
	identityAudit := identity.AuditorFunc(func(ctx context.Context, e identity.AuditEvent) error {
		return auditService.Record(ctx, audit.Event{ActorID: e.ActorID, Action: e.Action, Outcome: e.Outcome, ResourceType: e.ResourceType, ResourceID: e.ResourceID, Metadata: e.Metadata})
	})
	svc := identity.NewService(pool, cipher, identityAudit, cfg.Security)
	u, err := svc.CreateAdmin(context.Background(), *email, *name, password)
	if err != nil {
		fatal(err)
	}
	if err = authorization.NewService(pool).GrantAdministrator(context.Background(), u.ID); err != nil {
		fatal(err)
	}
	_ = auditService.Record(context.Background(), audit.Event{ActorID: u.ID, Action: "authorization.role.granted", Outcome: "success", ResourceType: "role", ResourceID: authorization.AdministratorRoleID, Metadata: map[string]any{"user_id": u.ID}})
	fmt.Printf("administrador criado: %s (%s)\n", u.Email, u.ID)
}
func fatal(err error) { slog.Error("command failed", "error", err); os.Exit(1) }
