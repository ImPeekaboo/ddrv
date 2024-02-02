package web

import (
	"embed"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

//go:embed static
var static embed.FS

func Load(app *fiber.App) {

	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(static),
		PathPrefix: "static",
	}))

	// For development purpose
	//app.Use(filesystem.New(filesystem.Config{
	//	Root: http.Dir("./internal/http/web/static"),
	//}))
}
