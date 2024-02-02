package dataprovider

import (
	"time"

	"github.com/forscht/ddrv/pkg/ns"
)

type File struct {
	Id     string        `json:"id"`
	Name   string        `json:"name" validate:"required,regex=^[\p{L}]+$"`
	Dir    bool          `json:"dir"`
	Size   int64         `json:"size,omitempty"`
	Parent ns.NullString `json:"parent,omitempty" validate:"required"`
	MTime  time.Time     `json:"mtime"`
}
