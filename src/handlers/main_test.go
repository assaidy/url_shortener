package handlers_test

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/assaidy/moon"
	"github.com/assaidy/url_shortener/src/routes"
	auth_service "github.com/assaidy/url_shortener/src/services/auth"
	postgres_service "github.com/assaidy/url_shortener/src/services/postgres"
	redirection_service "github.com/assaidy/url_shortener/src/services/redirection"
	redis_service "github.com/assaidy/url_shortener/src/services/redis"
	shortening_service "github.com/assaidy/url_shortener/src/services/shortening"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const (
	templateDBName = "template_db"
)

var (
	adminConnStr string
	redisConnStr string
	redisDBPool  chan int
	pgDBCounter  atomic.Uint64
)

func TestMain(m *testing.M) {
	os.Exit(runTestMain(m))
}

func runTestMain(m *testing.M) int {
	flag.Parse()

	limit, err := resolveTestParallelism(flag.CommandLine, runtime.GOMAXPROCS(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	pgContainer, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
		return 1
	}
	defer func() {
		if err := pgContainer.Terminate(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "failed to terminate postgres container: %v\n", err)
		}
	}()

	adminConnStr, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get postgres connection string: %v\n", err)
		return 1
	}

	if err := migrateTemplate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare template database: %v\n", err)
		return 1
	}

	redisContainer, err := tcredis.Run(ctx, "redis:7-alpine",
		testcontainers.WithCmd("redis-server", "--databases", strconv.Itoa(limit)),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start redis container: %v\n", err)
		return 1
	}
	defer func() {
		if err := redisContainer.Terminate(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "failed to terminate redis container: %v\n", err)
		}
	}()

	redisConnStr, err = redisContainer.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get redis connection string: %v\n", err)
		return 1
	}

	redisDBPool = make(chan int, limit)
	for i := range limit {
		redisDBPool <- i
	}

	return m.Run()
}

// resolveTestParallelism reports how many tests run in parallel. Go defaults
// -parallel to GOMAXPROCS; an explicit value always wins.
func resolveTestParallelism(fs *flag.FlagSet, gomaxprocs int) (int, error) {
	f := fs.Lookup("test.parallel")
	if f == nil {
		return gomaxprocs, nil
	}

	explicit := false
	fs.Visit(func(fl *flag.Flag) {
		if fl.Name == "test.parallel" {
			explicit = true
		}
	})
	if !explicit {
		return gomaxprocs, nil
	}

	n, err := strconv.Atoi(f.Value.String())
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid -parallel value %q", f.Value.String())
	}
	return n, nil
}

func migrateTemplate(ctx context.Context) error {
	adminDB, err := sql.Open("postgres", replaceDBName(adminConnStr, "postgres"))
	if err != nil {
		return fmt.Errorf("failed to connect to admin database: %w", err)
	}
	defer adminDB.Close()

	if _, err := adminDB.ExecContext(ctx, "CREATE DATABASE "+templateDBName); err != nil {
		return fmt.Errorf("failed to create template database: %w", err)
	}

	templateDB, err := sql.Open("postgres", replaceDBName(adminConnStr, templateDBName))
	if err != nil {
		return fmt.Errorf("failed to connect to template database: %w", err)
	}
	defer templateDB.Close()

	if err := templateDB.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping template database: %w", err)
	}

	root, err := moduleRoot()
	if err != nil {
		return err
	}
	dirs, err := filepath.Glob(filepath.Join(root, "src", "services", "*", "db", "migrations"))
	if err != nil {
		return fmt.Errorf("failed to find migration directories: %w", err)
	}
	if len(dirs) == 0 {
		return fmt.Errorf("no migration directories found under %s", root)
	}

	for _, dir := range dirs {
		service := filepath.Base(filepath.Dir(filepath.Dir(dir)))
		provider, err := goose.NewProvider(
			goose.DialectPostgres,
			templateDB,
			os.DirFS(dir),
			goose.WithTableName("goose_db_version_"+service),
		)
		if err != nil {
			return fmt.Errorf("failed to create goose provider for %s: %w", service, err)
		}
		if _, err := provider.Up(ctx); err != nil {
			return fmt.Errorf("failed to migrate %s: %w", service, err)
		}
	}

	return nil
}

func moduleRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

func replaceDBName(connStr, dbName string) string {
	u, err := url.Parse(connStr)
	if err != nil {
		panic(err)
	}
	u.Path = "/" + dbName
	return u.String()
}

func setupTestDB(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	adminDB, err := sql.Open("postgres", replaceDBName(adminConnStr, "postgres"))
	require.NoError(t, err)
	defer adminDB.Close()

	dbName := fmt.Sprintf("test_%d", pgDBCounter.Add(1))
	_, err = adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", dbName, templateDBName))
	require.NoError(t, err)

	dbURL := replaceDBName(adminConnStr, dbName)
	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	require.NoError(t, db.PingContext(ctx))

	t.Cleanup(func() {
		db.Close()

		cleanAdminDB, err := sql.Open("postgres", replaceDBName(adminConnStr, "postgres"))
		if err != nil {
			return
		}
		defer cleanAdminDB.Close()

		cleanAdminDB.ExecContext(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName))
	})

	return dbURL
}

func setupTestRedisDB(t *testing.T) (string, int) {
	t.Helper()
	ctx := context.Background()

	db := <-redisDBPool

	opts, err := redis.ParseURL(redisConnStr)
	require.NoError(t, err)
	opts.DB = db
	client := redis.NewClient(opts)
	require.NoError(t, client.Ping(ctx).Err())

	t.Cleanup(func() {
		client.FlushDB(context.Background()).Err()
		client.Close()
		redisDBPool <- db
	})

	u, err := url.Parse(redisConnStr)
	require.NoError(t, err)

	return u.Host, db
}

func setupTestApp(t *testing.T) *moon.App {
	t.Helper()

	app, _, _ := setupTestAppWithDB(t)
	return app
}

// setupTestAppWithDB is setupTestApp plus the PostgreSQL URL backing the app and
// the recorder that captures logs emitted through the app logger. Tests that
// need to inspect side effects which are not exposed by an endpoint can open
// their own connection to that database.
func setupTestAppWithDB(t *testing.T) (*moon.App, string, *logRecorder) {
	t.Helper()

	postgresUrl := setupTestDB(t)
	redisAddress, redisDb := setupTestRedisDB(t)

	logs := newLogRecorder()
	app := moon.New(
		moon.WithLogger(logs.Logger()),
		moon.WithRequestLogging(false),
	)
	app.AddDependency(logs.Logger())

	app.AddService(postgres_service.New(postgresUrl))
	app.AddService(redis_service.New(redisAddress, redisDb))
	app.AddService(auth_service.New())
	app.AddService(shortening_service.New())
	app.AddService(redirection_service.New())

	routes.Map(app)

	require.NoError(t, app.StartServices())
	t.Cleanup(app.StopServices)

	return app, postgresUrl, logs
}
