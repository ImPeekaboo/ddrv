package api

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"path/filepath"

	"github.com/gofiber/fiber/v2"

	dp "github.com/forscht/ddrv/internal/dataprovider"
	"github.com/forscht/ddrv/pkg/ddrv"
	"github.com/forscht/ddrv/pkg/httprange"
	"github.com/forscht/ddrv/pkg/lreader"
	"github.com/forscht/ddrv/pkg/ns"
)

func GetFileHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		dirId := c.Params("dirId")

		file, err := dp.Get(id, dirId)
		if err != nil {
			if errors.Is(err, dp.ErrNotExist) {
				return fiber.NewError(StatusNotFound, err.Error())
			}
			return err
		}
		return c.Status(StatusOk).
			JSON(Response{Message: "file retrieved", Data: file})
	}
}

func CreateFileHandler(driver *ddrv.Driver) fiber.Handler {
	return func(c *fiber.Ctx) error {
		dirId := c.Params("dirId")
		body := c.Context().RequestBodyStream()
		_, params, err := mime.ParseMediaType(string(c.Request().Header.ContentType()))
		if err != nil {
			return fiber.NewError(StatusBadRequest, ErrBadRequest)
		}

		boundary, ok := params["boundary"]
		if !ok {
			return fiber.NewError(StatusBadRequest, ErrBadRequest)
		}

		mreader := multipart.NewReader(body, boundary)

		for {
			part, err := mreader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if part.FormName() == "file" {
				fileName := part.FileName()
				if err = validate.Struct(dp.File{Name: fileName, Parent: ns.NullString(dirId)}); err != nil {
					return fiber.NewError(StatusBadRequest, err.Error())
				}

				file, err := dp.Create(fileName, dirId, false)
				if err != nil {
					if errors.Is(err, dp.ErrExist) || err == dp.ErrInvalidParent {
						return fiber.NewError(StatusBadRequest, err.Error())
					}
					return err
				}

				nodes := make([]ddrv.Node, 0)

				var dwriter io.WriteCloser
				onChunk := func(a ddrv.Node) {
					file.Size += int64(a.Size)
					nodes = append(nodes, a)
				}

				if c.Locals("asyncwrite").(bool) {
					dwriter = driver.NewNWriter(onChunk)
				} else {
					dwriter = driver.NewWriter(onChunk)
				}

				if _, err = io.Copy(dwriter, part); err != nil {
					return err
				}

				if err = dwriter.Close(); err != nil {
					return err
				}

				if err = dp.CreateNodes(file.Id, nodes); err != nil {
					return err
				}

				return c.Status(StatusOk).
					JSON(Response{Message: "file created", Data: file})
			}
		}

		return fiber.NewError(StatusBadRequest, ErrBadRequest)
	}
}

func UpdateFileHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		dirId := c.Params("dirId")

		file := new(dp.File)

		if err := c.BodyParser(file); err != nil {
			return fiber.NewError(StatusBadRequest, ErrBadRequest)
		}

		if err := validate.Struct(file); err != nil {
			return fiber.NewError(StatusBadRequest, err.Error())
		}

		file, err := dp.Update(id, dirId, file)
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
			JSON(Response{Message: "file updated", Data: file})
	}
}

func DelFileHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		dirId := c.Params("dirId")

		if err := dp.Delete(id, dirId); err != nil {
			if errors.Is(err, dp.ErrNotExist) {
				return fiber.NewError(StatusNotFound, err.Error())
			}
		}
		return c.Status(StatusOk).
			JSON(Response{Message: "file deleted"})
	}
}

func DownloadFileHandler(driver *ddrv.Driver) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		name := c.Params("fname")

		f, err := dp.Get(id, "")
		if err != nil {
			if errors.Is(err, dp.ErrNotExist) {
				return fiber.NewError(StatusNotFound, err.Error())
			}
			return err
		}

		if f.Name != name {
			c.Set(fiber.HeaderContentDisposition, "attachment; filename="+f.Name)
		} else {
			ext := filepath.Ext(name)
			mimeType := mime.TypeByExtension(ext)
			if mimeType == "" {
				mimeType = fiber.MIMEOctetStream
			}
			c.Set(fiber.HeaderContentType, mimeType)
		}

		nodes, err := dp.GetNodes(id)
		if err != nil {
			return err
		}

		fileRange := c.Request().Header.Peek("range")
		if fileRange != nil {
			r, err := httprange.Parse(string(fileRange), f.Size)
			if err != nil {
				return fiber.NewError(StatusRangeNotSatisfiable, err.Error())
			}

			c.Response().Header.Set("Content-Range", r.Header)

			dreader, err := driver.NewReader(nodes, r.Start)
			if err != nil {
				return err
			}
			c.Status(StatusPartialContent).Response().SetBodyStream(lreader.New(dreader, int(r.Length)), int(r.Length))
		} else {
			c.Set(fiber.HeaderAcceptRanges, "bytes")
			dreader, err := driver.NewReader(nodes, 0)
			if err != nil {
				return err
			}
			c.Status(StatusOk).Response().SetBodyStream(dreader, int(f.Size))
		}

		return err
	}
}
