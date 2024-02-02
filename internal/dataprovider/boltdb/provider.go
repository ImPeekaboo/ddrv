package boltdb

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"path"
	"path/filepath"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/rs/zerolog/log"
	"go.etcd.io/bbolt"

	dp "github.com/forscht/ddrv/internal/dataprovider"
	"github.com/forscht/ddrv/pkg/ddrv"
	"github.com/forscht/ddrv/pkg/locker"
)

const RootDirPath = "/"

type Provider struct {
	db     *bbolt.DB
	sg     *snowflake.Node
	driver *ddrv.Driver
	locker *locker.Locker
}

type Config struct {
	DbPath string `mapstructure:"db_path"`
}

func New(driver *ddrv.Driver, cfg *Config) dp.DataProvider {
	db, err := bbolt.Open(cfg.DbPath, 0666, nil)
	if err != nil {
		log.Fatal().Str("c", "boltdb").Err(err).Msg("failed to open db")
	}
	// Initialize the filesystem root
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err = tx.CreateBucketIfNotExists([]byte("fs")); err != nil {
			return err
		}
		if _, err = tx.CreateBucketIfNotExists([]byte("nodes")); err != nil {
			return err
		}
		rootData := serializeFile(dp.File{Name: "/", Dir: true, MTime: time.Now()})
		return tx.Bucket([]byte("fs")).Put([]byte(RootDirPath), rootData)
	})
	if err != nil {
		log.Fatal().Str("c", "boltdb").Err(err).Msg("failed to init db")
	}
	sg, err := snowflake.NewNode(int64(rand.Intn(1023)))
	if err != nil {
		log.Fatal().Err(err).Str("c", "boltdb").Msg("failed to create snowflake node")
	}
	log.Info().Str("c", "boltdb").Str("path", cfg.DbPath).Msg("initialized boltdb as dataprovider")

	return &Provider{db, sg, driver, locker.New()}
}

func (bfp *Provider) Name() string {
	return "boltdb"
}

func (bfp *Provider) Get(id, parent string) (*dp.File, error) {
	p := decodep(id)
	file, err := bfp.Stat(p)
	if err != nil {
		return nil, err
	}
	if parent != "" && string(file.Parent) != parent {
		return nil, dp.ErrNotExist
	}
	_, file.Name = filepath.Split(file.Name)
	return file, err
}

func (bfp *Provider) Update(id, parent string, file *dp.File) (*dp.File, error) {
	p := decodep(id)
	if p == RootDirPath {
		return nil, dp.ErrPermission
	}
	exciting, err := bfp.Stat(p)
	if err != nil {
		return nil, err
	}
	if parent != "" && string(exciting.Parent) != parent {
		return nil, dp.ErrInvalidParent
	}
	newp := path.Clean(decodep(string(file.Parent)) + "/" + file.Name)
	if err = bfp.Mv(exciting.Name, newp); err != nil {
		return nil, err
	}
	file.Name = newp
	return file, nil
}

func (bfp *Provider) GetChild(id string) ([]*dp.File, error) {
	p := decodep(id)
	file, err := bfp.Stat(p)
	if err != nil {
		return nil, err
	}
	if !file.Dir {
		return nil, dp.ErrInvalidParent
	}
	files, err := bfp.Ls(p, 0, 0)
	if err != nil {
		return nil, err
	}
	for _, file = range files {
		_, file.Name = filepath.Split(file.Name)
	}
	return files, nil
}

func (bfp *Provider) Create(name, parent string, dir bool) (*dp.File, error) {
	p := path.Clean(decodep(parent) + "/" + name)
	file := dp.File{Name: p, Dir: dir, MTime: time.Now()}
	err := bfp.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("fs"))
		existingFile := b.Get([]byte(p))
		if existingFile != nil {
			return dp.ErrExist
		}
		return b.Put([]byte(p), serializeFile(file))
	})
	file.Id = encodep(p)
	file.Name = name
	return &file, err
}

func (bfp *Provider) Delete(id, parent string) error {
	p := decodep(id)
	if p == RootDirPath {
		return dp.ErrPermission
	}
	file, err := bfp.Stat(p)
	if err != nil {
		return err
	}
	if parent != "" && string(file.Parent) != parent {
		return dp.ErrInvalidParent
	}
	return bfp.Rm(p)
}

func (bfp *Provider) GetNodes(id string) ([]ddrv.Node, error) {
	bfp.locker.Acquire(id)
	defer bfp.locker.Release(id)
	var nodes []ddrv.Node
	expired := make([]*ddrv.Node, 0)
	currentTimestamp := int(time.Now().Unix())
	err := bfp.db.Update(func(tx *bbolt.Tx) error {
		// Get the bucket for the specific file
		nodesBucket := tx.Bucket([]byte("nodes"))
		bucket := nodesBucket.Bucket([]byte(decodep(id)))
		if bucket == nil {
			return nil
		}
		if err := bucket.ForEach(func(k, v []byte) error {
			var node ddrv.Node
			deserializeNode(&node, v)
			if currentTimestamp > node.Ex {
				expired = append(expired, &node)
			}
			nodes = append(nodes, node)
			return nil
		}); err != nil {
			return err
		}
		if err := bfp.driver.UpdateNodes(expired); err != nil {
			return err
		}
		for _, node := range expired {
			data := serializeNode(*node)
			key := []byte(fmt.Sprintf("%d", node.NId))
			if err := bucket.Put(key, data); err != nil {
				return err
			}
		}
		return nil
	})

	return nodes, err
}

func (bfp *Provider) CreateNodes(id string, nodes []ddrv.Node) error {
	return bfp.db.Update(func(tx *bbolt.Tx) error {
		file, err := bfp.Stat(decodep(id))
		if err != nil {
			return dp.ErrNotExist
		}
		nodesBucket := tx.Bucket([]byte("nodes"))
		bucket, err := nodesBucket.CreateBucketIfNotExists([]byte(decodep(id)))
		if err != nil {
			return err
		}
		for _, node := range nodes {
			seq := bfp.sg.Generate()
			node.NId = seq.Int64()
			file.Size += int64(node.Size)
			data := serializeNode(node)
			if err = bucket.Put(seq.Bytes(), data); err != nil {
				return err
			}
		}
		data := serializeFile(*file)
		fs := tx.Bucket([]byte("fs"))
		return fs.Put([]byte(file.Name), data)
	})
}

// Truncate Removes all nodes for file if nodes found, does not return error if nodes not found
func (bfp *Provider) Truncate(id string) error {
	return bfp.db.Update(func(tx *bbolt.Tx) error {
		nodes := tx.Bucket([]byte("nodes"))
		err := nodes.DeleteBucket([]byte(decodep(id)))
		if errors.Is(err, bbolt.ErrBucketNotFound) {
			return nil
		}
		return err
	})
}

func (bfp *Provider) Stat(p string) (*dp.File, error) {
	p = path.Clean(p)
	var file *dp.File
	err := bfp.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("fs"))
		data := b.Get([]byte(p))
		if data == nil {
			return dp.ErrNotExist
		}
		file = deserializeFile(data)
		return nil
	})
	return file, err
}

func (bfp *Provider) Ls(p string, limit int, offset int) ([]*dp.File, error) {
	p = path.Clean(p)
	var files []*dp.File
	err := bfp.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("fs"))
		c := b.Cursor()
		prefix := []byte(p)
		var skipped, collected int
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			// Skip the root path itself
			if string(k) == p || !findDirectChild(p, string(k)) {
				continue
			}
			if limit > 0 && collected >= limit {
				break
			}
			if skipped < offset {
				skipped++
				continue
			}

			files = append(files, deserializeFile(v))
			collected++
		}
		return nil
	})
	return files, err
}

func (bfp *Provider) Touch(p string) error {
	p = path.Clean(p)
	return bfp.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("fs"))
		existingFile := b.Get([]byte(p))
		// If the file does not exist, create it
		if existingFile == nil {
			data := serializeFile(dp.File{Name: p, Dir: false, MTime: time.Now()})
			return b.Put([]byte(p), data)
		}
		return nil
	})
}

func (bfp *Provider) Mkdir(p string) error {
	p = path.Clean(p)

	return bfp.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("fs"))
		// Iterate through parent directories and create them if they don't exist.
		for dir := p; dir != "." && dir != "/"; dir = filepath.Dir(dir) {
			exciting := b.Get([]byte(dir))
			if exciting == nil {
				// Directory does not exist, create it.
				data := serializeFile(dp.File{Name: dir, Dir: true, MTime: time.Now()})
				if err := b.Put([]byte(dir), data); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (bfp *Provider) Rm(p string) error {
	p = path.Clean(p)
	return bfp.db.Update(func(tx *bbolt.Tx) error {
		fs := tx.Bucket([]byte("fs"))
		nodes := tx.Bucket([]byte("nodes"))
		// Check if the directory exists
		data := fs.Get([]byte(p))
		if data == nil {
			return dp.ErrNotExist
		}
		// Delete the specified directory
		if err := fs.Delete([]byte(p)); err != nil {
			return err
		}
		// Check if the file is dir or not
		// if the file is not directory then remove nodes and return
		file := deserializeFile(data)
		if !file.Dir {
			err := nodes.DeleteBucket([]byte(decodep(file.Id)))
			if errors.Is(err, bbolt.ErrBucketNotFound) {
				return nil
			}
			return err
		}
		// Delete all children in the directory
		prefix := []byte(p + "/")
		c := fs.Cursor()
		var filesToDelete [][]byte
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			filesToDelete = append(filesToDelete, k)
		}
		for _, f := range filesToDelete {
			if err := fs.Delete(f); err != nil {
				return err
			}
			err := nodes.DeleteBucket(f)
			if err != nil && !errors.Is(err, bbolt.ErrBucketNotFound) {
				return err
			}
		}
		return nil
	})
}

func (bfp *Provider) Mv(oldPath, newPath string) error {
	oldPath = path.Clean(oldPath)
	newPath = path.Clean(newPath)
	return bfp.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("fs"))
		if exist := b.Get([]byte(newPath)); exist != nil {
			return dp.ErrExist
		}
		// Move the specified file or directory
		data := b.Get([]byte(oldPath))
		if data == nil {
			return dp.ErrNotExist
		}
		if err := bfp.RenameFile(tx, b, data, oldPath, newPath); err != nil {
			return err
		}
		// Move all children in the directory
		prefix := []byte(oldPath + "/")
		newPrefix := []byte(newPath + "/")
		c := b.Cursor()
		var filesToMove [][]byte
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			filesToMove = append(filesToMove, k)
		}
		for _, f := range filesToMove {
			newKey := append(newPrefix, f[len(prefix):]...)
			if err := bfp.RenameFile(tx, b, b.Get(f), string(f), string(newKey)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (bfp *Provider) RenameFile(tx *bbolt.Tx, b *bbolt.Bucket, data []byte, oldp, newp string) error {
	file := deserializeFile(data)
	file.Name = newp
	if err := b.Delete([]byte(oldp)); err != nil {
		return err
	}
	if err := b.Put([]byte(newp), serializeFile(*file)); err != nil {
		return err
	}
	if !file.Dir {
		return bfp.RenameBucket(tx, oldp, newp)
	}
	return nil
}

func (bfp *Provider) RenameBucket(tx *bbolt.Tx, oldp, newp string) error {
	nodesBucket := tx.Bucket([]byte("nodes"))
	oldBucket := nodesBucket.Bucket([]byte(oldp))
	if oldBucket != nil {
		newBucket, err := nodesBucket.CreateBucket([]byte(newp))
		if err != nil {
			return err
		}
		err = oldBucket.ForEach(func(k, v []byte) error {
			return newBucket.Put(k, v)
		})
		if err != nil {
			return err
		}
		if err = nodesBucket.DeleteBucket([]byte(oldp)); err != nil {
			return err
		}
	}
	return nil
}

func (bfp *Provider) CHTime(p string, newMTime time.Time) error {
	p = path.Clean(p)
	return bfp.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("fs"))
		fileData := b.Get([]byte(p))
		// Check if the file or directory exists
		if fileData == nil {
			return dp.ErrNotExist
		}
		// Deserialize the file data
		file := deserializeFile(fileData)
		// Update the modification time
		file.MTime = newMTime
		// Serialize the updated file data
		return b.Put([]byte(p), serializeFile(*file))
	})
}

func (bfp *Provider) Close() error {
	return bfp.db.Close()
}
