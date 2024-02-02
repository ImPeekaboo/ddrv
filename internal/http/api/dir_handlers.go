package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	dp "github.com/forscht/ddrv/internal/dataprovider"
)

func GetDirHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		dir, err := dp.Get(id, "")
		if err != nil {
			if errors.Is(err, dp.ErrNotExist) {
				return fiber.NewError(StatusNotFound, err.Error())
			}
			return err
		}
		files, err := dp.GetChild(c.Params("id"))
		if err != nil {
			return err
		}
		directory := Directory{dir, files}
		return c.Status(StatusOk).
			JSON(Response{Message: "directory retrieved", Data: directory})
	}
}

func CreateDirHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		file := new(dp.File)

		if err := c.BodyParser(file); err != nil {
			return fiber.NewError(StatusBadRequest, ErrBadRequest)
		}

		if err := validate.Struct(file); err != nil {
			return fiber.NewError(StatusBadRequest, err.Error())
		}

		file, err := dp.Create(file.Name, string(file.Parent), true)
		if err != nil {
			if errors.Is(err, dp.ErrExist) || err == dp.ErrInvalidParent {
				return fiber.NewError(StatusBadRequest, err.Error())
			}
			return err
		}
		return c.Status(StatusCreated).
			JSON(Response{Message: "directory created", Data: file})
	}
}

func UpdateDirHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")

		dir := new(dp.File)

		if err := c.BodyParser(dir); err != nil {
			return fiber.NewError(StatusBadRequest, ErrBadRequest)
		}

		if err := validate.Struct(dir); err != nil {
			return fiber.NewError(StatusBadRequest, err.Error())
		}

		dir, err := dp.Update(id, "", dir)
		if err != nil {
			if errors.Is(err, dp.ErrNotExist) {
				return fiber.NewError(StatusNotFound, err.Error())
			}
			if errors.Is(err, dp.ErrExist) {
				return fiber.NewError(StatusBadRequest, err.Error())
			}
			return err
		}

		return c.Status(StatusOk).
			JSON(Response{Message: "directory updated", Data: dir})
	}
}

func DelDirHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")

		if err := dp.Delete(id, ""); err != nil {
			if errors.Is(err, dp.ErrPermission) {
				return fiber.NewError(StatusForbidden, err.Error())
			}
			if errors.Is(err, dp.ErrNotExist) {
				return fiber.NewError(StatusNotFound, err.Error())
			}
			return err
		}
		return c.Status(StatusOk).
			JSON(Response{Message: "directory deleted"})
	}
}
