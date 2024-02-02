package filesystem

import (
	"io"
	"os"
	"path/filepath"
	"time"

	dp "github.com/forscht/ddrv/internal/dataprovider"
	"github.com/forscht/ddrv/pkg/ddrv"
)

type File struct {
	id    string
	name  string
	dir   bool
	size  int64
	mtime time.Time

	flag         int
	off          int64
	data         []ddrv.Node
	readDirCount int

	fs          *Fs
	driver      *ddrv.Driver
	chunks      []ddrv.Node
	streamWrite io.WriteCloser
	streamRead  io.ReadCloser
}

func (f *File) Size() int64                { return f.size }
func (f *File) ModTime() time.Time         { return f.mtime }
func (f *File) IsDir() bool                { return f.dir }
func (f *File) Sys() interface{}           { return nil }
func (f *File) Stat() (os.FileInfo, error) { return f, nil }
func (f *File) Sync() error                { return nil }

func (f *File) Truncate(_ int64) error                 { return ErrNotSupported }
func (f *File) WriteAt(_ []byte, _ int64) (int, error) { return 0, ErrNotSupported }

func (f *File) Name() string {
	_, name := filepath.Split(f.name)
	if name == "" {
		return "/"
	}
	return name
}

func (f *File) Mode() os.FileMode {
	if f.IsDir() {
		return os.ModeDir | 0755 // Set directory mode
	}
	return 0444 // Set regular file mode
}

func (f *File) Readdirnames(n int) ([]string, error) {
	if !f.IsDir() {
		return nil, ErrIsNotDir
	}
	fi, err := f.Readdir(n)
	names := make([]string, len(fi))
	for i, f := range fi {
		_, names[i] = filepath.Split(f.Name())
	}

	return names, err
}

func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	if !f.IsDir() {
		return nil, ErrIsNotDir
	}

	files, err := dp.Ls(f.name, count, f.readDirCount)
	if err != nil {
		return nil, err
	}
	entries := make([]os.FileInfo, len(files))
	for i, file := range files {
		entries[i] = f.fs.convertToAferoFile(file)
	}
	if count > 0 && len(entries) == 0 {
		err = io.EOF
	}
	f.readDirCount += len(entries)

	return entries, err
}

func (f *File) Read(p []byte) (n int, err error) {
	if f.IsDir() {
		return 0, ErrIsDir
	}
	if f.streamRead == nil {
		if err = f.openReadStream(0); err != nil {
			return 0, err
		}
	}
	n, err = f.streamRead.Read(p)
	// Do not increment n on failed read
	if err != nil && err != io.EOF {
		return n, err
	}
	f.off += int64(n)
	return n, err
}

func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	if f.IsDir() {
		return 0, ErrIsDir
	}
	if _, err := f.Seek(off, io.SeekCurrent); err != nil {
		return 0, err
	}
	return f.Read(p)
}

func (f *File) WriteString(s string) (ret int, err error) {
	if f.IsDir() {
		return 0, ErrIsDir
	}
	return f.Write([]byte(s))
}

func (f *File) Write(p []byte) (int, error) {
	if f.IsDir() {
		return 0, ErrIsDir
	}

	if !CheckFlag(os.O_WRONLY, f.flag) {
		return 0, ErrNotSupported
	}

	if f.streamWrite == nil {
		if CheckFlag(os.O_APPEND, f.flag) {
			if err := dp.Truncate(f.id); err != nil {
				return 0, err
			}
		}
		if f.fs.asyncWrite {
			f.streamWrite = f.driver.NewNWriter(func(chunk ddrv.Node) {
				f.chunks = append(f.chunks, chunk)
			})
		} else {
			f.streamWrite = f.driver.NewWriter(func(chunk ddrv.Node) {
				f.chunks = append(f.chunks, chunk)
			})
		}
	}
	n, err := f.streamWrite.Write(p)

	return n, err
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.IsDir() {
		return 0, ErrIsDir
	}

	if !CheckFlag(os.O_RDONLY, f.flag) {
		return 0, ErrNotSupported
	}

	pos := int64(0)

	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = f.off + offset
	case io.SeekEnd:
		pos = f.Size() - offset
	}
	if pos < 0 {
		return 0, ErrInvalidSeek
	}
	if f.streamRead != nil {
		if err := f.streamRead.Close(); err != nil {
			return 0, err
		}
	}
	f.streamRead = nil
	if err := f.openReadStream(pos); err != nil {
		return 0, err
	}

	return pos, nil
}

func (f *File) Close() error {
	if f.streamWrite != nil {
		if err := f.streamWrite.Close(); err != nil {
			return err
		}
		// Special case, some FTP clients try to create blank file
		// and then try to write it to FTP, we can ignore chunks with 0 bytes
		if len(f.chunks) == 1 && f.chunks[0].Size == 0 {
			return nil
		}
		err := dp.CreateNodes(f.id, f.chunks)
		if err != nil {
			return err
		}
		f.streamWrite = nil
	}
	if f.streamRead != nil {
		if err := f.streamRead.Close(); err != nil {
			return err
		}
		f.streamRead = nil
	}

	return nil
}

func (f *File) openReadStream(startAt int64) error {
	stream, err := f.driver.NewReader(f.data, startAt)
	if err != nil {
		return err
	}
	f.streamRead = stream
	return nil
}
