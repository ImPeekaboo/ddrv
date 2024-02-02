package boltdb

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"path"
	"path/filepath"

	"github.com/rs/zerolog/log"

	dp "github.com/forscht/ddrv/internal/dataprovider"
	"github.com/forscht/ddrv/pkg/ddrv"
	"github.com/forscht/ddrv/pkg/ns"
)

func serializeNode(node ddrv.Node) []byte {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(node)
	if err != nil {
		log.Fatal().Str("c", "boltdb provider").Err(err).Msg("failed to serialize node")
	}
	return buffer.Bytes()
}

func deserializeNode(node *ddrv.Node, data []byte) {
	buffer := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buffer)
	err := dec.Decode(node)
	if err != nil {
		log.Fatal().Str("c", "boltdb provider").Err(err).Msg("failed to deserialize file")
	}
}

func serializeFile(file dp.File) []byte {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(file)
	if err != nil {
		log.Fatal().Str("c", "boltdb provider").Err(err).Msg("failed to serialize file")
	}
	return buffer.Bytes()
}

func deserializeFile(data []byte) *dp.File {
	file := new(dp.File)
	buffer := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buffer)
	err := dec.Decode(file)
	if err != nil {
		log.Fatal().Str("c", "boltdb provider").Err(err).Msg("failed to deserialize file")
	}
	file.Id = encodep(file.Name)
	if file.Name != "/" {
		parent, _ := filepath.Split(file.Name)
		file.Parent = ns.NullString(encodep(parent))
	}
	return file
}

func decodep(id string) string {
	decoded, err := base64.StdEncoding.DecodeString(id)
	if err != nil {
		log.Fatal().Str("c", "boltdb provider").Err(err).Msg("failed to decode base64")
	}
	// Convert the bytes to a string and print it
	p := string(decoded)
	if p == "" {
		p = "/"
	}
	return path.Clean(p)
}

func encodep(p string) string {
	p = path.Clean(p)
	return base64.StdEncoding.EncodeToString([]byte(p))
}

// findDirectChild checks if arg2 is a direct child of arg1.
func findDirectChild(arg1, arg2 string) bool {
	// Split the child path into directory and file name components.
	dir, _ := filepath.Split(arg2)
	// The Split function leaves a trailing slash on the directory component,
	// so we need to clean it again to make it comparable with arg1.
	dir = path.Clean(dir)
	// Check if the directory part of arg2 matches arg1.
	return dir == arg1
}
