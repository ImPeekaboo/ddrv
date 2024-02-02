package filesystem

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/spf13/afero"
)

// LogFsFile is a wrapper to log interactions around file accesses
type LogFsFile struct {
	src           afero.File // Source file
	lengthRead    int        // Length read
	lengthWritten int        // Length written
}

// LogFs is a wrapper to log interactions around file system accesses
type LogFs struct {
	src    afero.Fs // Source file system
	logger zerolog.Logger
}

// NewLogFs creates an instance with logging
func NewLogFs(src afero.Fs) afero.Fs {
	return &LogFs{src, log.With().Str("c", "fs").Logger()}
}

func (lf *LogFs) log(err error) *zerolog.Event {
	l := lf.logger.Debug()
	if err != nil {
		l = log.Error().Err(err)
	}
	return l
}

// Create calls will be logged
func (lf *LogFs) Create(name string) (afero.File, error) {
	src, err := lf.src.Create(name)
	lf.log(err).Str("name", name).Msg("CREATE")
	return &LogFsFile{src: src}, err
}

// Mkdir calls will be logged
func (lf *LogFs) Mkdir(name string, perm os.FileMode) error {
	err := lf.src.Mkdir(name, perm)
	lf.log(err).Str("name", name).Any("fmode", perm).Msg("MKDIR")
	return err
}

// MkdirAll calls will be logged
func (lf *LogFs) MkdirAll(path string, perm os.FileMode) error {
	err := lf.src.MkdirAll(path, perm)
	lf.log(err).Any("fmode", perm).Str("path", path).Msg("MKDIR_ALL")
	return err
}

// Open calls will be logged
func (lf *LogFs) Open(name string) (afero.File, error) {
	src, err := lf.src.Open(name)
	lf.log(err).Str("name", name).Msg("OPEN")
	if err != nil {
		return src, err
	}
	return &LogFsFile{src: src}, err
}

// OpenFile calls will be logged
func (lf *LogFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	src, err := lf.src.OpenFile(name, flag, perm)
	lf.log(err).Str("name", name).Any("fmode", perm).Int("flag", flag).Msg("OPEN_FILE")
	if err != nil {
		return src, err
	}
	return &LogFsFile{src: src}, err
}

// Remove calls will be logged
func (lf *LogFs) Remove(name string) error {
	err := lf.src.Remove(name)
	lf.log(err).Str("name", name).Msg("REMOVE")
	return err
}

// RemoveAll calls will be logged
func (lf *LogFs) RemoveAll(path string) error {
	err := lf.src.RemoveAll(path)
	lf.log(err).Str("path", path).Msg("REMOVE_ALL")
	return err
}

// Rename calls will not be logged
func (lf *LogFs) Rename(oldname, newname string) error {
	err := lf.src.Rename(oldname, newname)
	lf.log(err).Str("newname", newname).Str("oldname", oldname).Msg("RENAME")
	return err
}

// Stat calls will not be logged
func (lf *LogFs) Stat(name string) (os.FileInfo, error) {
	return lf.src.Stat(name)
}

// Name calls will not be logged
func (lf *LogFs) Name() string {
	return lf.src.Name()
}

// Chmod calls will not be logged
func (lf *LogFs) Chmod(name string, mode os.FileMode) error {
	return lf.src.Chmod(name, mode)
}

// Chtimes calls will not be logged
func (lf *LogFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return lf.src.Chtimes(name, atime, mtime)
}

// Chown calls will not be logged
func (lf *LogFs) Chown(name string, uid int, gid int) error {
	return lf.src.Chown(name, uid, gid)
}

// Close calls will be logged
func (lff *LogFsFile) Close() error {
	err := lff.src.Close()
	l := log.Debug()
	if err != nil {
		l = log.Error().Err(err)
	}
	l.Str("c", "fs").Str("name", lff.src.Name()).
		Int("lread", lff.lengthRead).Int("lwrite", lff.lengthWritten).Msg("CLOSE")
	return err
}

// Read only log error
func (lff *LogFsFile) Read(p []byte) (int, error) {
	n, err := lff.src.Read(p)
	if err == nil {
		lff.lengthRead += n
	}
	if err != nil && err != io.EOF {
		log.Error().Str("c", "fs").Str("name", lff.Name()).Err(err).Msg("READ")
	}
	return n, err
}

// ReadAt only log error
func (lff *LogFsFile) ReadAt(p []byte, off int64) (int, error) {
	n, err := lff.src.ReadAt(p, off)
	if err == nil {
		lff.lengthRead += n
	}
	if err != nil && err != io.EOF {
		log.Error().Str("c", "fs").Str("name", lff.Name()).Int64("off", off).Err(err).Msg("READ_AT")
	}
	return n, err
}

// Seek only log error
func (lff *LogFsFile) Seek(offset int64, whence int) (int64, error) {
	n, err := lff.src.Seek(offset, whence)
	if err != nil {
		log.Error().Str("c", "fs").Str("name", lff.Name()).Int64("off", offset).
			Int("whence", whence).Err(err).Msg("SEEK")
	}
	return n, err
}

// Write only log error
func (lff *LogFsFile) Write(p []byte) (int, error) {
	n, err := lff.src.Write(p)
	if err == nil {
		lff.lengthWritten += n
	}
	if err != nil {
		log.Error().Str("c", "fs").Str("name", lff.Name()).Err(err).Msg("WRITE")
	}
	return n, err
}

// WriteAt only log error
func (lff *LogFsFile) WriteAt(p []byte, off int64) (int, error) {
	n, err := lff.src.WriteAt(p, off)
	if err == nil {
		lff.lengthWritten += n
	}
	if err != nil {
		log.Error().Str("c", "fs").Str("name", lff.Name()).Int64("off", off).Err(err).Msg("WRITE_AT")
	}
	return n, err
}

// WriteString only log error
func (lff *LogFsFile) WriteString(str string) (int, error) {
	n, err := lff.src.WriteString(str)
	if err == nil {
		lff.lengthWritten += n
	}
	if err != nil {
		log.Error().Str("c", "fs").Str("name", lff.Name()).Err(err).Msg("WRITE_STRING")
	}
	return n, err
}

// Name won't be logged
func (lff *LogFsFile) Name() string {
	return lff.src.Name()
}

// Readdir won't be logged
func (lff *LogFsFile) Readdir(count int) ([]os.FileInfo, error) {
	info, err := lff.src.Readdir(count)
	l := log.Debug()
	if err != nil {
		l = log.Error().Err(err)
	}
	l.Str("c", "fs").Str("name", lff.Name()).Int("count", count).Msg("READ_DIR")
	return info, err
}

func (lff *LogFsFile) Readdirnames(n int) ([]string, error) {
	names, err := lff.src.Readdirnames(n)
	l := log.Debug()
	if err != nil {
		l = log.Error().Err(err)
	}
	l.Str("c", "fs").Str("name", lff.Name()).Int("count", n).Msg("READ_DIR_NAMES")
	return names, err
}

func (lff *LogFsFile) Stat() (os.FileInfo, error) {
	info, err := lff.src.Stat()
	l := log.Debug()
	if err != nil {
		l = log.Error().Err(err)
	}
	l.Str("c", "fs").Str("name", lff.Name()).Msg("STAT")
	return info, err
}

func (lff *LogFsFile) Sync() error {
	err := lff.src.Sync()
	l := log.Debug()
	if err != nil {
		l = log.Error().Err(err)
	}
	l.Str("c", "fs").Str("name", lff.Name()).Msg("SYNC")
	return err
}

func (lff *LogFsFile) Truncate(size int64) error {
	err := lff.src.Truncate(size)
	l := log.Debug()
	if err != nil {
		l = log.Error().Err(err)
	}
	l.Str("c", "fs").Int64("size", size).Str("name", lff.Name()).Msg("TRUNCATE")
	return err
}
