package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"carryapi/internal/api"
	"carryapi/internal/apikey"
	"carryapi/internal/auth"
	"carryapi/internal/catalog"
	"carryapi/internal/config"
	"carryapi/internal/crypto"
	"carryapi/internal/db"
	"carryapi/internal/proxy"
	"carryapi/internal/server"
	"carryapi/internal/settings"
	"carryapi/internal/stats"
	"carryapi/internal/user"
	"carryapi/internal/webauthn"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// 构造所有 store/handler
	cipher := crypto.NewCipherOrPanic(cfg.MasterKey)
	us := user.New(d, cipher)
	ss := auth.NewSessionStore(d)
	st := settings.New(d)
	ks := apikey.New(d)
	ls := auth.NewLoginService(us, ss, st)
	authH := api.NewAuthHandler(ls, ss, us, st)
	usersH := api.NewUserHandler(us, ss)
	setupH := api.NewSetupHandler(us)
	keysH := api.NewKeyHandler(ks)
	quotasH := api.NewQuotaHandler(us)
	settingsH := api.NewSettingsHandler(st)
	oauthH := api.NewOAuthHandler(us, ss, st)

	// catalog(上游 provider/模型/价格 管理)
	catProv := catalog.NewProviderStore(d, cipher)
	catModel := catalog.NewModelStore(d)
	catPrice := catalog.NewPriceStore(d)
	catalogH := catalog.NewHandler(catProv, catModel, catPrice)

	// WebAuthn (passkey) Relying Party config. Defaults target local dev
	// (localhost:8067); override via env for production deployments.
	rpID := os.Getenv("CARRYAPI_RP_ID")
	if rpID == "" {
		rpID = "localhost"
	}
	rpOrigin := os.Getenv("CARRYAPI_RP_ORIGIN")
	if rpOrigin == "" {
		rpOrigin = fmt.Sprintf("http://localhost:%d", cfg.Port)
	}
	passkeySvc, err := webauthn.New(rpID, rpOrigin)
	if err != nil {
		log.Fatalf("webauthn init: %v", err)
	}
	passkeyH := api.NewPasskeyHandler(passkeySvc, us, ss)

	// 首次启动:若无 admin 则创建
	bootstrapAdmin(d, us)

	// 上游代理(catalog 的 store 直接注入)
	proxyInstance := proxy.NewProxy(proxy.Deps{
		DB: d, Keys: ks, Users: us,
		Models: catModel, Providers: catProv, Prices: catPrice,
	})

	// 统计/管理 API handlers
	statsH := stats.NewHandler(d)

	srv := server.New(cfg, server.Deps{
		DB:       d,
		Store:    st,
		Users:    us,
		Sessions: ss,
		Auth:     authH,
		UsersH:   usersH,
		Setup:    setupH,
		Keys:     keysH,
		Quotas:   quotasH,
		Settings: settingsH,
		OAuth:    oauthH,
		Passkey:  passkeyH,
		Catalog:  catalogH,
		Proxy:    proxyInstance,
		Stats:    statsH,
	})

	// 信号处理
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		fmt.Println("\nshutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("serve: %v", err)
	}
}

// bootstrapAdmin creates an admin only when both CARRYAPI_ADMIN_EMAIL and
// CARRYAPI_ADMIN_PASSWORD are explicitly set AND no admin exists yet. This
// preserves scripted/provisioned deployments. Otherwise setup is left to the
// first-run browser wizard (/api/setup/admin). No random password is printed.
func bootstrapAdmin(d *sql.DB, us *user.Store) {
	email := os.Getenv("CARRYAPI_ADMIN_EMAIL")
	pw := os.Getenv("CARRYAPI_ADMIN_PASSWORD")
	if email == "" || pw == "" {
		return
	}
	has, err := us.HasAdmin()
	if err != nil {
		log.Printf("admin count check: %v", err)
		return
	}
	if has {
		return
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		log.Printf("hash admin password: %v", err)
		return
	}
	if _, err := us.Create(email, hash, "admin"); err != nil {
		log.Printf("create admin: %v", err)
	}
}
