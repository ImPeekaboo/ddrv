package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	zl "github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	dp "github.com/forscht/ddrv/internal/dataprovider"
	"github.com/forscht/ddrv/internal/dataprovider/boltdb"
	"github.com/forscht/ddrv/internal/dataprovider/postgres"
	"github.com/forscht/ddrv/internal/ftp"
	"github.com/forscht/ddrv/internal/http"
	"github.com/forscht/ddrv/pkg/ddrv"
)

// Config represents the entire configuration as defined in the YAML file.
type Config struct {
	Ddrv struct {
		Tokens    []string `mapstructure:"token"`
		TokenType int      `mapstructure:"token_type"`
		Channels  []string `mapstructure:"channels"`
		ChunkSize int      `mapstructure:"chunk_size"`
		Nitro     bool     `mapstructure:"nitro"`
	} `mapstructure:"ddrv"`

	Dataprovider struct {
		Bolt     boltdb.Config   `mapstructure:"boltdb"`
		Postgres postgres.Config `mapstructure:"postgres"`
	} `mapstructure:"dataprovider"`

	Frontend struct {
		FTP  ftp.Config  `mapstructure:"ftp"`
		HTTP http.Config `mapstructure:"http"`
	} `mapstructure:"frontend"`
}

var config Config

var (
	showVersion = flag.Bool("version", false, "print version information and exit")
	debugMode   = flag.Bool("debug", false, "enable debug logs")
	configFile  = flag.String("config", "", "path to ddrv configuration file")
)

func main() {
	flag.Parse()

	// Check if a version flag is set
	if *showVersion {
		fmt.Printf("ddrv: %s\n", version)
		os.Exit(0)
	}

	// Set the maximum number of operating system threads to use.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Setup logger
	log.Logger = zl.New(zl.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
	zl.SetGlobalLevel(zl.InfoLevel)
	if *debugMode {
		zl.SetGlobalLevel(zl.DebugLevel)
	}

	// Load config file
	initConfig()

	// Create a ddrv driver
	driver, err := ddrv.New((*ddrv.Config)(&config.Ddrv))
	if err != nil {
		log.Fatal().Err(err).Str("c", "main").Msg("failed to open ddrv driver")
	}

	// Load data provider
	var provider dp.DataProvider
	if config.Dataprovider.Bolt.DbPath != "" {
		provider = boltdb.New(driver, &config.Dataprovider.Bolt)
	}
	if provider == nil && config.Dataprovider.Postgres.DbURL != "" {
		provider = postgres.New(&config.Dataprovider.Postgres, driver)
	}
	if provider == nil {
		log.Fatal().Str("c", "main").Msg("dataprovider config is missing")
	}
	dp.Load(provider)

	errCh := make(chan error)
	// Create and start ftp server
	go func() { errCh <- ftp.Serv(driver, &config.Frontend.FTP) }()
	// Create and start http server
	go func() { errCh <- http.Serv(driver, &config.Frontend.HTTP) }()

	if err = <-errCh; err != nil {
		log.Fatal().Str("c", "main").Err(err).Msgf("ddrv crashed")
	}
}

func initConfig() {
	// Setup config
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/ddrv/")
	if *configFile != "" {
		viper.SetConfigFile(*configFile)
	}
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Str("c", "config").Err(err).Msg("failed to read config")
	}

	// Bind env
	_ = viper.BindEnv("ddrv.token", "TOKEN")
	_ = viper.BindEnv("ddrv.token_type", "TOKEN_TYPE")
	_ = viper.BindEnv("ddrv.channels", "CHANNELS")
	_ = viper.BindEnv("ddrv.nitro", "NITRO")
	_ = viper.BindEnv("ddrv.chunk_size", "CHUNK_SIZE")

	_ = viper.BindEnv("dataprovider.boltdb.db_path", "BOLTDB_DB_PATH")
	_ = viper.BindEnv("dataprovider.postgres.db_url", "POSTGRES_DB_URL")

	_ = viper.BindEnv("frontend.ftp.addr", "FTP_ADDR")
	_ = viper.BindEnv("frontend.ftp.username", "FTP_USERNAME")
	_ = viper.BindEnv("frontend.ftp.password", "FTP_PASSWORD")
	_ = viper.BindEnv("frontend.ftp.async_write", "FTP_ASYNC_WRITE")
	_ = viper.BindEnv("frontend.http.addr", "HTTP_ADDR")
	_ = viper.BindEnv("frontend.http.username", "HTTP_USERNAME")
	_ = viper.BindEnv("frontend.http.password", "HTTP_PASSWORD")
	_ = viper.BindEnv("frontend.http.guest_mode", "HTTP_GUEST_MODE")
	_ = viper.BindEnv("frontend.http.async_write", "HTTP_ASYNC_WRITE")
	_ = viper.BindEnv("frontend.http.https_addr", "HTTPS_ADDR")
	_ = viper.BindEnv("frontend.http.https_crtpath", "HTTPS_CRTPATH")
	_ = viper.BindEnv("frontend.http.https_keypath", "HTTPS_KEYPATH")

	err := viper.Unmarshal(&config)
	if err != nil {
		log.Fatal().Str("c", "config").Err(err).Msg("failed to decode config into struct")
	}
}
