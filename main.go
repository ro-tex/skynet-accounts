package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/NebulousLabs/skynet-accounts/api"
	"github.com/NebulousLabs/skynet-accounts/build"
	"github.com/NebulousLabs/skynet-accounts/database"
	"github.com/NebulousLabs/skynet-accounts/jwt"
	"github.com/NebulousLabs/skynet-accounts/metafetcher"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v71"
	"gitlab.com/NebulousLabs/errors"
)

const (
	// defaultPortal is the URL of the default Skynet portal, maintained by
	// Skynet Labs. It can be overridden by an environment variable.
	defaultPortal = "https://siasky.net"
)

var (
	// envDBHost holds the name of the environment variable for DB host.
	envDBHost = "SKYNET_DB_HOST"
	// envDBPort holds the name of the environment variable for DB port.
	envDBPort = "SKYNET_DB_PORT"
	// envDBUser holds the name of the environment variable for DB username.
	envDBUser = "SKYNET_DB_USER"
	// envDBPass holds the name of the environment variable for DB password.
	envDBPass = "SKYNET_DB_PASS" // #nosec G101: Potential hardcoded credentials
	// envLogLevel holds the name of the environment variable which defines the
	// desired log level.
	envLogLevel = "SKYNET_ACCOUNTS_LOG_LEVEL"
	// envPortal holds the name of the environment variable for the portal to
	// use to fetch skylinks.
	envPortal = "PORTAL_URL"
	// envKratosAddr hold the name of the environment variable for Kratos's
	// address. Defaults to "kratos:4433".
	envKratosAddr = "KRATOS_ADDR"
	// envOathkeeperAddr hold the name of the environment variable for
	// Oathkeeper's address. Defaults to "oathkeeper:4456".
	envOathkeeperAddr = "OATHKEEPER_ADDR"
	// envStripeApiKey hold the name of the environment variable for Stripe's
	// API key. It's only required when integrating with Stripe.
	envStripeApiKey = "STRIPE_API_KEY"
)

// loadDBCredentials creates a new DB connection based on credentials found in
// the environment variables.
func loadDBCredentials() (database.DBCredentials, error) {
	var cds database.DBCredentials
	var ok bool
	if cds.User, ok = os.LookupEnv(envDBUser); !ok {
		return database.DBCredentials{}, errors.New("missing env var " + envDBUser)
	}
	if cds.Password, ok = os.LookupEnv(envDBPass); !ok {
		return database.DBCredentials{}, errors.New("missing env var " + envDBPass)
	}
	if cds.Host, ok = os.LookupEnv(envDBHost); !ok {
		return database.DBCredentials{}, errors.New("missing env var " + envDBHost)
	}
	if cds.Port, ok = os.LookupEnv(envDBPort); !ok {
		return database.DBCredentials{}, errors.New("missing env var " + envDBPort)
	}
	return cds, nil
}

func main() {
	// Load the environment variables from the .env file.
	// Existing variables take precedence and won't be overwritten.
	_ = godotenv.Load()
	portal, ok := os.LookupEnv(envPortal)
	if !ok {
		portal = defaultPortal
	}
	dbCreds, err := loadDBCredentials()
	if err != nil {
		log.Fatal(errors.AddContext(err, "failed to fetch DB credentials"))
	}
	if kaddr := os.Getenv(envKratosAddr); kaddr != "" {
		jwt.KratosAddr = kaddr
	}
	if oaddr := os.Getenv(envOathkeeperAddr); oaddr != "" {
		jwt.OathkeeperAddr = oaddr
	}
	if sk := os.Getenv(envStripeApiKey); sk != "" {
		stripe.Key = sk
		api.StripeTestMode = !strings.HasPrefix(stripe.Key, "sk_live_")
	}

	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logLevel())
	db, err := database.New(ctx, dbCreds, logger)
	if err != nil {
		log.Fatal(errors.AddContext(err, "failed to connect to the DB"))
	}
	mf := metafetcher.New(ctx, db, portal, logger)
	server, err := api.New(db, mf, logger)
	if err != nil {
		log.Fatal(errors.AddContext(err, "failed to build the API"))
	}
	logger.Info("Listening on port 3000")
	logger.Fatal(http.ListenAndServe(":3000", server.Router()))
}

// logLevel returns the desires log level.
func logLevel() logrus.Level {
	switch debugEnv, _ := os.LookupEnv(envLogLevel); debugEnv {
	case "panic":
		return logrus.PanicLevel
	case "fatal":
		return logrus.FatalLevel
	case "error":
		return logrus.ErrorLevel
	case "warn":
		return logrus.WarnLevel
	case "info":
		return logrus.InfoLevel
	case "debug":
		return logrus.DebugLevel
	case "trace":
		return logrus.TraceLevel
	}
	if build.DEBUG {
		return logrus.TraceLevel
	}
	if build.Release == "testing" || build.Release == "dev" {
		return logrus.DebugLevel
	}
	return logrus.InfoLevel
}
