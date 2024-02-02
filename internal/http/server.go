package http

import (
	"errors"

	fzl "github.com/gofiber/contrib/fiberzerolog"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/rs/zerolog/log"

	"github.com/forscht/ddrv/internal/http/api"
	"github.com/forscht/ddrv/internal/http/web"
	"github.com/forscht/ddrv/pkg/ddrv"
)

type Config struct {
	Addr         string `mapstructure:"addr"`
	HTTPSAddr    string `mapstructure:"https_addr"`
	HTTPSKeyPath string `mapstructure:"https_keypath"`
	HTTPSCrtPath string `mapstructure:"https_crtpath"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	GuestMode    bool   `mapstructure:"guest_mode"`
	AsyncWrite   bool   `mapstructure:"async_write"`
}

func Serv(driver *ddrv.Driver, cfg *Config) error {

	fconfig := fiber.Config{
		DisablePreParseMultipartForm: true, // https://github.com/gofiber/fiber/issues/1838
		StreamRequestBody:            true,
		DisableStartupMessage:        true,
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError // Status code defaults to 500
			if ctx.BaseURL() == "http://" || ctx.BaseURL() == "https://" {
				return nil
			}
			// Retrieve the custom status code if it's a *fiber.Error
			var e *fiber.Error
			if errors.As(err, &e) {
				code = e.Code
			}
			if code != fiber.StatusInternalServerError {
				return ctx.Status(code).JSON(api.Response{Message: err.Error()})
			}
			return ctx.Status(code).JSON(api.Response{Message: "internal server error"})
		},
	}

	// Initialize fiber app
	app := fiber.New(fconfig)

	// Setup config vars
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("username", cfg.Username)
		c.Locals("password", cfg.Password)
		c.Locals("guestmode", cfg.GuestMode)
		c.Locals("asyncwrite", cfg.AsyncWrite)
		return c.Next()
	})

	// Enable logger
	logger := log.With().Str("c", "httpserver").Logger()
	app.Use(fzl.New(fzl.Config{Logger: &logger}))

	// Enable cors
	app.Use(cors.New())

	// Load Web routes
	web.Load(app)

	// Register API routes
	api.Load(app, driver)

	// Error channel to capture any listen errors
	errChan := make(chan error)

	// Listen on HTTP
	go func() {
		if cfg.Addr != "" {
			log.Info().Str("c", "http").Str("addr", cfg.Addr).Msg("starting http server")
			errChan <- app.Listen(cfg.Addr)
		}
	}()

	// Listen on HTTPS
	go func() {
		if cfg.HTTPSAddr != "" && cfg.HTTPSCrtPath != "" && cfg.HTTPSKeyPath != "" {
			log.Info().Str("c", "http").Str("addr", cfg.HTTPSAddr).Msg("starting https server")
			errChan <- app.ListenTLS(cfg.HTTPSAddr, cfg.HTTPSCrtPath, cfg.HTTPSKeyPath)
		}
	}()

	// Return the first error received
	return <-errChan
}
