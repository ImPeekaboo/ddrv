package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/afero"

	dp "github.com/forscht/ddrv/internal/dataprovider"
	"github.com/forscht/ddrv/pkg/ddrv"
)

var (
	ErrIsDir        = &os.PathError{Err: errors.New("is a directory")}
	ErrIsNotDir     = &os.PathError{Err: errors.New("is not a directory")}
	ErrNotSupported = &os.PathError{Err: errors.New("fs doesn't support this operation")}
	ErrInvalidSeek  = &os.PathError{Err: errors.New("invalid seek offset")}
	ErrReadOnly     = os.ErrPermission
)

type Fs struct {
	driver     *ddrv.Driver
	asyncWrite bool
}

func New(driver *ddrv.Driver, asyncWrite bool) afero.Fs {
	return NewLogFs(&Fs{driver, asyncWrite})
}

func (fs *Fs) Name() string                        { return "LogFs" }
func (fs *Fs) Chown(_ string, _, _ int) error      { return ErrNotSupported }
func (fs *Fs) Chmod(_ string, _ os.FileMode) error { return ErrNotSupported }
func (fs *Fs) Chtimes(name string, _ time.Time, mtime time.Time) error {
	return dp.ChMTime(name, mtime)
}

func (fs *Fs) Create(name string) (afero.File, error) {
	if err := dp.Touch(name); err != nil {
		return nil, err
	}
	return fs.OpenFile(name, os.O_WRONLY, 0666)
}

func (fs *Fs) Mkdir(name string, _ os.FileMode) error {
	parent, _ := filepath.Split(name)
	file, err := dp.Stat(parent)
	if err != nil {
		return err
	}
	if !file.Dir {
		return ErrIsNotDir
	}
	err = dp.Mkdir(name)
	return err
}

func (fs *Fs) MkdirAll(path string, _ os.FileMode) error {
	err := dp.Mkdir(path)
	return err
}

func (fs *Fs) Open(name string) (afero.File, error) {
	f, err := dp.Stat(name)
	if err != nil {
		return nil, err
	}
	file := fs.convertToAferoFile(f)
	file.flag = os.O_RDONLY
	file.driver = fs.driver
	if !file.dir {
		file.data, err = dp.GetNodes(file.id)
		if err != nil {
			return nil, err
		}
	}

	return file, nil
}

// OpenFile supported flags, O_WRONLY, O_CREATE, O_RDONLY
func (fs *Fs) OpenFile(name string, flag int, _ os.FileMode) (afero.File, error) {

	if !CheckFlag(flag, os.O_WRONLY|os.O_RDONLY|os.O_CREATE|os.O_TRUNC) {
		return nil, ErrReadOnly
	}

	// If record not found and os.O_CREATE flag is enabled
	f, err := dp.Stat(name)
	if err != nil {
		if CheckFlag(os.O_CREATE, flag) {
			return fs.Create(name)
		}
		return nil, err
	}

	file := fs.convertToAferoFile(f)
	file.flag = flag
	file.driver = fs.driver

	if CheckFlag(os.O_TRUNC, flag) {
		if err = dp.Truncate(file.id); err != nil {
			return nil, err
		}
	}

	if !file.dir {
		file.data, err = dp.GetNodes(file.id)
		if err != nil {
			return nil, err
		}
	}

	return file, nil
}

func (fs *Fs) Remove(name string) error {
	parent, _ := filepath.Split(name)
	_, err := dp.Stat(parent)
	if err != nil {
		return err
	}
	return dp.Rm(name)
}

func (fs *Fs) RemoveAll(path string) error {
	return dp.Rm(path)
}

func (fs *Fs) Rename(oldname, newname string) error {
	return dp.Mv(oldname, newname)
}

func (fs *Fs) Stat(name string) (os.FileInfo, error) {
	f, err := dp.Stat(name)
	if err != nil {
		return nil, os.ErrNotExist
	}
	return fs.convertToAferoFile(f).Stat()
}

func CheckFlag(flag int, allowedFlags int) bool {
	return flag == (flag & allowedFlags)
}

func (fs *Fs) convertToAferoFile(df *dp.File) *File {
	return &File{id: df.Id, name: df.Name, dir: df.Dir, size: df.Size, mtime: df.MTime, fs: fs}
}
