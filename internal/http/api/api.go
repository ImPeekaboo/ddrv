package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/forscht/ddrv/internal/dataprovider"
	"github.com/forscht/ddrv/pkg/ddrv"
	"github.com/forscht/ddrv/pkg/validator"
)

var validate = validator.New()

func Load(app *fiber.App, driver *ddrv.Driver) {

	// create api API group
	api := app.Group("/api")

	// public route for public login
	api.Post("/user/login", LoginHandler())

	// returns necessary ddrv auth config
	api.Get("/config", AuthConfigHandler())

	// setup auth middleware
	api.Use(AuthHandler())

	// verify JWT token (required on a page load)
	api.Get("/check_token", CheckTokenHandler())

	// If dataprovider is postgres, we require id and dirId to be guid
	if dataprovider.Name() == "postgres" {
		// Load directory middlewares
		api.Post("/directories/", CreateDirHandler())
		api.Get("/directories/:id<guid>?", GetDirHandler())
		api.Put("/directories/:id<guid>", UpdateDirHandler())
		api.Delete("/directories/:id<guid>", DelDirHandler())

		// Load file middlewares
		api.Post("/directories/:dirId<guid>/files", CreateFileHandler(driver))
		api.Get("/directories/:dirId<guid>/files/:id<guid>", GetFileHandler())
		api.Put("/directories/:dirId<guid>/files/:id<guid>", UpdateFileHandler())
		api.Delete("/directories/:dirId<guid>/files/:id<guid>", DelFileHandler())

		// Just like discord, we will not authorize file endpoints
		// so that it can work with download managers or media players
		app.Get("/files/:id<guid>", DownloadFileHandler(driver))
		app.Get("/files/:id<guid>/:fname", DownloadFileHandler(driver))

		return
	}

	// Load directory middlewares
	api.Post("/directories/", CreateDirHandler())
	api.Get("/directories/:id?", GetDirHandler())
	api.Put("/directories/:id", UpdateDirHandler())
	api.Delete("/directories/:id", DelDirHandler())

	// Load file middlewares
	api.Post("/directories/:dirId/files", CreateFileHandler(driver))
	api.Get("/directories/:dirId/files/:id", GetFileHandler())
	api.Put("/directories/:dirId/files/:id", UpdateFileHandler())
	api.Delete("/directories/:dirId/files/:id", DelFileHandler())

	// Just like discord, we will not authorize file endpoints
	// so that it can work with download managers or media players
	app.Get("/files/:id", DownloadFileHandler(driver))
	app.Get("/files/:id/:fname", DownloadFileHandler(driver))
}
