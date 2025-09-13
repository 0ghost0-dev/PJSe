package app

import (
	"context"
	"sync"

	"PJS_Exchange/databases"
	"PJS_Exchange/databases/postgres"
)

type App struct {
	DB           *databases.PostgresDBPool
	Repositories *Repositories
}

type Repositories struct {
	User       *postgres.UserDBRepository
	Symbol     *postgres.SymbolDBRepository
	APIKey     *postgres.APIKeyDBRepository
	AcceptCode *postgres.AcceptCodeDBRepository
}

var (
	appInstance *App
	appOnce     sync.Once
)

func GetApp() *App {
	appOnce.Do(func() {
		appInstance = initializeApp()
	})
	return appInstance
}

func initializeApp() *App {
	ctx := context.Background()

	postgresDB, err := databases.NewPostgresDBPool(databases.NewPostgresDBConfig())
	if err != nil {
		panic("Failed to connect to Postgres DB: " + err.Error())
	}

	acceptRepo := postgres.NewAcceptCodeRepository(postgresDB)
	userRepo := postgres.NewUserRepository(postgresDB, acceptRepo)
	symbolRepo := postgres.NewSymbolRepository(postgresDB)
	apikeyRepo := postgres.NewAPIKeyRepository(postgresDB)

	repos := &Repositories{
		AcceptCode: acceptRepo,
		User:       userRepo,
		Symbol:     symbolRepo,
		APIKey:     apikeyRepo,
	}

	if err := createTables(ctx, repos); err != nil {
		panic("Failed to create tables: " + err.Error())
	}

	return &App{
		DB:           postgresDB,
		Repositories: repos,
	}
}

func createTables(ctx context.Context, repos *Repositories) error {
	if err := repos.User.CreateUsersTable(ctx); err != nil {
		return err
	}
	if err := repos.Symbol.CreateSymbolsTable(ctx); err != nil {
		return err
	}
	if err := repos.APIKey.CreateAPIKeysTable(ctx); err != nil {
		return err
	}
	if err := repos.AcceptCode.CreateAcceptCodesTable(ctx); err != nil {
		return err
	}
	return nil
}

func (app *App) UserRepo() *postgres.UserDBRepository     { return app.Repositories.User }
func (app *App) SymbolRepo() *postgres.SymbolDBRepository { return app.Repositories.Symbol }
func (app *App) APIKeyRepo() *postgres.APIKeyDBRepository { return app.Repositories.APIKey }
func (app *App) AcceptCodeRepo() *postgres.AcceptCodeDBRepository {
	return app.Repositories.AcceptCode
}

func (app *App) Close() {
	if app.DB != nil {
		app.DB.Close()
	}
}
