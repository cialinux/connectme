package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/connectme/connectme/core/config"
	"github.com/connectme/connectme/core/database"
	"github.com/connectme/connectme/core/encryption"
	"github.com/connectme/connectme/core/events"
	"github.com/connectme/connectme/core/httpserver"
	"github.com/connectme/connectme/core/metrics"
	"github.com/connectme/connectme/core/modules"
	"github.com/connectme/connectme/modules/audit"
	"github.com/connectme/connectme/modules/authorization"
	"github.com/connectme/connectme/modules/connections"
	"github.com/connectme/connectme/modules/credentials"
	"github.com/connectme/connectme/modules/hosts"
	"github.com/connectme/connectme/modules/identity"
	"github.com/connectme/connectme/modules/locations"
	"github.com/connectme/connectme/modules/remote"
	"github.com/connectme/connectme/modules/system"
	"github.com/jackc/pgx/v5/pgxpool"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type App struct {
	Config          config.Config
	Log             *slog.Logger
	DB              *pgxpool.Pool
	Registry        *modules.Registry
	HTTP            *httpserver.Server
	shutdownTimeout time.Duration
}

func Build(ctx context.Context, c config.Config, log *slog.Logger, migrationFS fs.FS) (*App, error) {
	pool, err := database.Open(ctx, c.Database)
	if err != nil {
		return nil, err
	}
	if c.Database.MigrateOnStart {
		for _, name := range []string{"core", "audit", "identity", "authorization", "locations", "hosts", "credentials", "connections"} {
			ms, loadErr := database.LoadMigrations(migrationFS, name, name)
			if loadErr != nil {
				pool.Close()
				return nil, loadErr
			}
			if err = database.Migrate(ctx, pool, ms); err != nil {
				pool.Close()
				return nil, err
			}
		}
	}
	mux := http.NewServeMux()
	bus := events.New()
	reg := modules.New(mux, bus)
	if err = reg.Add(system.New(pool)); err != nil {
		pool.Close()
		return nil, err
	}
	cipher, err := encryption.New(c.Security.MasterKey)
	if err != nil {
		pool.Close()
		return nil, err
	}
	auditService := audit.NewService(pool, c.Security.AuditKey)
	identityAudit := identity.AuditorFunc(func(ctx context.Context, e identity.AuditEvent) error {
		return auditService.Record(ctx, audit.Event{ActorID: e.ActorID, Action: e.Action, Outcome: e.Outcome, ResourceType: e.ResourceType, ResourceID: e.ResourceID, CorrelationID: e.CorrelationID, SourceIP: e.SourceIP, Metadata: e.Metadata})
	})
	identityService := identity.NewService(pool, cipher, identityAudit, c.Security)
	authorizationService := authorization.NewService(pool)
	defaultAdmin, created, createErr := identityService.EnsureDefaultAdmin(ctx)
	if createErr != nil {
		pool.Close()
		return nil, createErr
	}
	if created {
		if err = authorizationService.GrantAdministrator(ctx, defaultAdmin.ID); err != nil {
			pool.Close()
			return nil, err
		}
		_ = auditService.Record(ctx, audit.Event{ActorID: defaultAdmin.ID, Action: "authorization.role.granted", Outcome: "success", ResourceType: "role", ResourceID: authorization.AdministratorRoleID})
	}
	identityModule := identity.New(identityService, c.Security, authorizationService)
	for _, m := range []modules.Module{audit.New(auditService), identityModule, authorization.New(authorizationService)} {
		if err = reg.Add(m); err != nil {
			pool.Close()
			return nil, err
		}
	}
	locationService := locations.NewService(pool)
	hostService := hosts.NewService(pool, locationService)
	credentialProvider := credentials.NewProvider(pool, cipher)
	connectionService := connections.NewService(pool, hostService, credentialProvider)
	for _, m := range []modules.Module{locations.New(locationService, identityModule), hosts.New(hostService, identityModule), credentials.New(credentialProvider, identityModule), connections.New(connectionService, identityModule)} {
		if err = reg.Add(m); err != nil {
			pool.Close()
			return nil, err
		}
	}
	if os.Getenv("CONNECTME_GUACAMOLE_KEY") != "" {
		rm, e := remote.New(identityService, identityModule, hostService, credentialProvider, connectionService)
		if e != nil {
			pool.Close()
			return nil, e
		}
		if e = reg.Add(rm); e != nil {
			pool.Close()
			return nil, e
		}
	}
	if err = reg.Resolve(c.Modules); err != nil {
		pool.Close()
		return nil, err
	}
	if err = reg.Register(); err != nil {
		pool.Close()
		return nil, err
	}
	mux.Handle(http.MethodGet+" /readyz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statuses := reg.Check(r.Context())
		code := http.StatusOK
		for name, s := range statuses {
			for _, d := range reg.Descriptors() {
				if d.Name == name && d.Critical && s.Status != "ok" {
					code = http.StatusServiceUnavailable
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(statuses)
	}))
	met := metrics.New()
	mux.Handle(http.MethodGet+" /metrics", met.Handler())
	handler := httpserver.Middleware(log, met.Middleware(mux))
	return &App{Config: c, Log: log, DB: pool, Registry: reg, HTTP: httpserver.New(c.HTTP, handler, log), shutdownTimeout: c.HTTP.ShutdownTimeout}, nil
}
func (a *App) Run(ctx context.Context) error {
	if err := a.Registry.Start(ctx); err != nil {
		return err
	}
	errCh := make(chan error, 1)
	go func() { errCh <- a.HTTP.Run() }()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		stopCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
		defer cancel()
		httpErr := a.HTTP.Shutdown(stopCtx)
		moduleErr := a.Registry.Stop(stopCtx)
		a.DB.Close()
		if httpErr != nil {
			return fmt.Errorf("http shutdown: %w", httpErr)
		}
		return moduleErr
	}
}
