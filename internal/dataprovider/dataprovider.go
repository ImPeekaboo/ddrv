package dataprovider

import (
	"time"

	"github.com/rs/zerolog/log"

	"github.com/forscht/ddrv/pkg/ddrv"
)

var provider DataProvider

type DataProvider interface {
	Name() string
	Get(id, parent string) (*File, error)
	GetChild(id string) ([]*File, error)
	Create(name, parent string, isDir bool) (*File, error)
	Update(id, parent string, file *File) (*File, error)
	Delete(id, parent string) error
	GetNodes(id string) ([]ddrv.Node, error)
	CreateNodes(id string, nodes []ddrv.Node) error
	Truncate(id string) error
	Stat(path string) (*File, error)
	Ls(path string, limit int, offset int) ([]*File, error)
	Touch(path string) error
	Mkdir(path string) error
	Rm(path string) error
	Mv(name, newname string) error
	CHTime(path string, time time.Time) error
	Close() error
}

func Load(dp DataProvider) {
	provider = dp
}

func Name() string {
	return provider.Name()
}
func Get(id, parent string) (*File, error) {
	log.Debug().Str("c", "dataprovider").Str("id", id).Str("parent", parent).Msg("GET")
	return provider.Get(id, parent)
}

func GetChild(id string) ([]*File, error) {
	log.Debug().Str("c", "dataprovider").Str("id", id).Msg("GET_CHILD")
	return provider.GetChild(id)
}

func Create(name, parent string, isDir bool) (*File, error) {
	log.Debug().Str("c", "dataprovider").Str("name", name).Str("parent", parent).Bool("dir", isDir).Msg("CREATE")
	return provider.Create(name, parent, isDir)
}

func Update(id, parent string, file *File) (*File, error) {
	log.Debug().Str("c", "dataprovider").Str("id", id).Str("parent", parent).Str("name", file.Name).Msg("UPDATE")
	return provider.Update(id, parent, file)
}

func Delete(id, parent string) error {
	log.Debug().Str("c", "dataprovider").Str("id", id).Str("parent", parent).Msg("DELETE")
	return provider.Delete(id, parent)
}

func GetNodes(fid string) ([]ddrv.Node, error) {
	log.Debug().Str("c", "dataprovider").Str("fid", fid).Msg("GET_NODES")
	return provider.GetNodes(fid)
}

func CreateNodes(fid string, nodes []ddrv.Node) error {
	log.Debug().Str("c", "dataprovider").Str("fid", fid).Msg("CREATE_NODES")
	return provider.CreateNodes(fid, nodes)
}

func Truncate(fid string) error {
	log.Debug().Str("c", "dataprovider").Str("fid", fid).Msg("TRUNCATE")
	return provider.Truncate(fid)
}

func Stat(path string) (*File, error) {
	log.Debug().Str("c", "dataprovider").Str("path", path).Msg("STAT")
	return provider.Stat(path)
}

func Ls(path string, limit int, offset int) ([]*File, error) {
	log.Debug().Str("c", "dataprovider").Str("path", path).Int("limit", limit).Int("off", offset).Msg("LS")
	return provider.Ls(path, limit, offset)
}

func Touch(path string) error {
	log.Debug().Str("c", "dataprovider").Str("path", path).Msg("TOUCH")
	return provider.Touch(path)
}

func Mkdir(path string) error {
	log.Debug().Str("c", "dataprovider").Str("path", path).Msg("MKDIR")
	return provider.Mkdir(path)
}

func Rm(path string) error {
	log.Debug().Str("c", "dataprovider").Str("path", path).Msg("RM")
	return provider.Rm(path)
}

func Mv(name, newname string) error {
	log.Debug().Str("c", "dataprovider").Str("name", name).Str("newname", newname).Msg("MV")
	return provider.Mv(name, newname)
}

func ChMTime(path string, t time.Time) error {
	log.Debug().Str("c", "dataprovider").Str("path", path).Time("time", t).Msg("CH_TIME")
	return provider.CHTime(path, t)
}
