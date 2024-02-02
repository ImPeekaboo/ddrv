package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"

	dp "github.com/forscht/ddrv/internal/dataprovider"
	"github.com/forscht/ddrv/pkg/ddrv"
	"github.com/forscht/ddrv/pkg/locker"
	"github.com/forscht/ddrv/pkg/ns"
)

const RootDirId = "11111111-1111-1111-1111-111111111111"

type PGProvider struct {
	db     *sql.DB
	sg     *snowflake.Node
	driver *ddrv.Driver
	locker *locker.Locker
}

type Config struct {
	DbURL string `mapstructure:"db_url"`
}

func New(cfg *Config, driver *ddrv.Driver) dp.DataProvider {
	// Create database connection
	dbConn := NewDb(cfg.DbURL, false)
	sg, err := snowflake.NewNode(int64(rand.Intn(1023)))
	if err != nil {
		log.Fatal().Err(err).Str("c", "postgres").Msg("failed to create snowflake node")
	}
	log.Info().Str("c", "postgres").Msg("initialized postgres as dataprovider")

	return &PGProvider{dbConn, sg, driver, locker.New()}
}

func (pgp *PGProvider) Name() string {
	return "postgres"
}

func (pgp *PGProvider) Get(id, parent string) (*dp.File, error) {
	file := new(dp.File)
	var err error
	if id == "" {
		id = RootDirId
	}
	if parent != "" {
		err = pgp.db.QueryRow(`
			SELECT id, name, dir, size, parent, mtime
			FROM fs
			WHERE fs.id=$1 AND parent=$2;
		`, id, parent).Scan(&file.Id, &file.Name, &file.Dir, &file.Size, &file.Parent, &file.MTime)
	} else {
		err = pgp.db.QueryRow(`
			SELECT id, name, dir, size, parent, mtime
			FROM fs
			WHERE fs.id=$1;
		`, id).Scan(&file.Id, &file.Name, &file.Dir, &file.Size, &file.Parent, &file.MTime)
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dp.ErrNotExist
		}
		return nil, err
	}

	return file, nil
}

func (pgp *PGProvider) GetChild(id string) ([]*dp.File, error) {
	_, err := pgp.Get(id, "")
	if err != nil {
		return nil, err
	}
	if id == "" {
		id = RootDirId
	}
	files := make([]*dp.File, 0)
	rows, err := pgp.db.Query(`
				SELECT id, name, dir, size, parent, mtime
				FROM fs
				WHERE fs.parent = $1
				ORDER BY fs.dir DESC, fs.name;
			`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		child := new(dp.File)
		if err := rows.Scan(&child.Id, &child.Name, &child.Dir, &child.Size, &child.Parent, &child.MTime); err != nil {
			return nil, err
		}
		files = append(files, child)
	}
	return files, nil
}

func (pgp *PGProvider) Create(name, parent string, dir bool) (*dp.File, error) {
	parentDir, err := pgp.Get(parent, "")
	if err != nil {
		return nil, err
	}
	if !parentDir.Dir {
		return nil, dp.ErrInvalidParent
	}
	file := &dp.File{Name: name, Parent: ns.NullString(parent)}
	if err = pgp.db.QueryRow("INSERT INTO fs (name,dir,parent) VALUES($1,$2,$3) RETURNING id, dir, mtime", name, dir, parent).
		Scan(&file.Id, &file.Dir, &file.MTime); err != nil {
		return nil, pqErrToOs(err) // Handle already exists
	}
	return file, pgp.refresh()
}

func (pgp *PGProvider) Update(id, parent string, file *dp.File) (*dp.File, error) {
	if id == RootDirId {
		return nil, dp.ErrPermission
	}
	var err error
	if parent == "" {
		err = pgp.db.QueryRow(
			"UPDATE fs SET name=$1, parent=$2, mtime = NOW() WHERE id=$3 RETURNING id,dir,mtime",
			file.Name, file.Parent, id,
		).Scan(&file.Id, &file.Dir, &file.MTime)
	} else {
		err = pgp.db.QueryRow(
			"UPDATE fs SET name=$1, parent=$2, mtime = NOW() WHERE id=$3 AND parent=$4 RETURNING id,dir,mtime",
			file.Name, file.Parent, id, parent,
		).Scan(&file.Id, &file.Dir, &file.MTime)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dp.ErrNotExist
		}
		return nil, pqErrToOs(err) // Handle already exists
	}
	return file, pgp.refresh()
}

func (pgp *PGProvider) Delete(id, parent string) error {
	if id == RootDirId {
		return dp.ErrPermission
	}
	var res sql.Result
	var err error
	if parent != "" {
		res, err = pgp.db.Exec("DELETE FROM fs WHERE id=$1 AND parent=$2", id, parent)
	} else {
		res, err = pgp.db.Exec("DELETE FROM fs WHERE id=$1", id)
	}

	if err != nil {
		return err
	}
	rAffected, _ := res.RowsAffected()
	if rAffected == 0 {
		return dp.ErrNotExist
	}
	return pgp.refresh()
}

func (pgp *PGProvider) GetNodes(id string) ([]ddrv.Node, error) {
	pgp.locker.Acquire(id)
	defer pgp.locker.Release(id)

	nodes := make([]ddrv.Node, 0)
	rows, err := pgp.db.Query(`SELECT url, size, mid, ex, "is", hm FROM node where file=$1 ORDER BY id ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	expired := make([]*ddrv.Node, 0)
	currentTimestamp := int(time.Now().Unix())
	for rows.Next() {
		var node ddrv.Node
		err = rows.Scan(&node.URL, &node.Size, &node.MId, &node.Ex, &node.Is, &node.Hm)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
		if currentTimestamp > node.Ex {
			expired = append(expired, &node)
		}
	}
	if err = pgp.driver.UpdateNodes(expired); err != nil {
		return nil, err
	}
	for _, node := range expired {
		if _, err = pgp.db.Exec(
			`UPDATE node SET ex=$1, "is"=$2, hm=$3 WHERE mid=$4`,
			node.Ex, node.Is, node.Hm, node.MId,
		); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

func (pgp *PGProvider) CreateNodes(fid string, nodes []ddrv.Node) error {
	// Nothing to do if there are no nodes provided
	if len(nodes) == 0 {
		return nil
	}
	tx, err := pgp.db.Begin()
	if err != nil {
		return err
	}
	// Defer a rollback in case anything goes wrong
	defer tx.Rollback()

	// Build the INSERT query with multiple values
	var values []interface{}
	query := `INSERT INTO node (id, file, url, size, mid, ex, "is", hm) VALUES`
	phc := 1 // placeHolderCounter
	for _, node := range nodes {
		id := pgp.sg.Generate()
		query += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d),", phc, phc+1, phc+2, phc+3, phc+4, phc+5, phc+6, phc+7)
		values = append(values, id, fid, node.URL, node.Size, node.MId, node.Ex, node.Is, node.Hm)
		phc += 8
	}
	// Remove the last comma and execute the query
	query = query[:len(query)-1]

	if _, err = tx.Exec(query, values...); err != nil {
		return err
	}

	// Update mtime every time something is written on file
	if _, err = tx.Exec(`
						UPDATE fs 
						SET size = COALESCE((SELECT SUM(size) FROM node WHERE node.file = fs.id), 0), mtime = NOW() 
						WHERE id = $1;
						`, fid); err != nil {
		return err
	}
	// If everything went well, commit the transaction
	if err = tx.Commit(); err != nil {
		return err
	}

	return pgp.refresh()
}

func (pgp *PGProvider) Truncate(fid string) error {
	if _, err := pgp.db.Exec("DELETE FROM node WHERE file=$1", fid); err != nil {
		return err
	}
	return pgp.refresh()
}

func (pgp *PGProvider) Stat(name string) (*dp.File, error) {
	file := new(dp.File)
	err := pgp.db.QueryRow("SELECT id, name, dir, size, mtime FROM stat($1)", name).
		Scan(&file.Id, &file.Name, &file.Dir, &file.Size, &file.MTime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dp.ErrNotExist
		}
		return nil, pqErrToOs(err)
	}
	return file, nil
}

func (pgp *PGProvider) Ls(name string, limit int, offset int) ([]*dp.File, error) {
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = pgp.db.Query("SELECT id, name, dir, size, mtime FROM ls($1) ORDER BY name limit $2 offset $3", name, limit, offset)
	} else {
		rows, err = pgp.db.Query("SELECT id, name, dir, size, mtime FROM ls($1) ORDER BY name", name)
	}
	if err != nil {
		return nil, pqErrToOs(err)
	}
	defer rows.Close()

	entries := make([]*dp.File, 0)
	for rows.Next() {
		file := new(dp.File)
		if err = rows.Scan(&file.Id, &file.Name, &file.Dir, &file.Size, &file.MTime); err != nil {
			return nil, err
		}
		entries = append(entries, file)
	}
	return entries, nil
}

func (pgp *PGProvider) Touch(name string) error {
	_, err := pgp.db.Exec("SELECT FROM touch($1)", name)
	return pqErrToOs(err)
}

func (pgp *PGProvider) Mkdir(name string) error {
	_, err := pgp.db.Exec("SELECT mkdir($1)", name)
	return pqErrToOs(err)
}

func (pgp *PGProvider) Rm(name string) error {
	_, err := pgp.db.Exec("SELECT rm($1)", name)
	return pqErrToOs(err)
}

func (pgp *PGProvider) Mv(name, newname string) error {
	_, err := pgp.db.Exec("SELECT mv($1, $2)", name, newname)
	return pqErrToOs(err)
}

func (pgp *PGProvider) CHTime(name string, mtime time.Time) error {
	if _, err := pgp.db.Exec("UPDATE fs SET mtime = $1 WHERE id=(SELECT id FROM stat($2));", mtime, name); err != nil {
		return pqErrToOs(err)
	}
	return pgp.refresh()
}

func (pgp *PGProvider) refresh() error {
	_, err := pgp.db.Exec("SELECT * FROM refresh_vfs();")
	return err
}

// Handle custom PGFs code
func pqErrToOs(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "P0001": // root dir permission issue
			return dp.ErrPermission
		case "P0002":
			return dp.ErrNotExist
		case "P0003":
			return dp.ErrExist
		case "P0004": // is not a directory
			return dp.ErrInvalidParent
		case "23505": // Unique violation error code
			return dp.ErrExist
		// Foreign key constraint violation occurred -> on CreateNodes
		// This error occurs when FTP clients try to do open -> remove -> close
		// Linux in case of os.OpenFile -> os.Remove -> file.Close ignores error, so we will too
		case "23503": //
			return nil
		default:
			return err
		}
	}
	return err
}

func (pgp *PGProvider) Close() error {
	return pgp.db.Close()
}
