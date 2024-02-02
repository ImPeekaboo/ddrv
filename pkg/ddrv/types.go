package ddrv

import "errors"

// ErrClosed is returned when a writer or reader is
// closed and caller is trying to read or write
var ErrClosed = errors.New("is closed")

// ErrAlreadyClosed is returned when the reader/writer is already closed
var ErrAlreadyClosed = errors.New("already closed")

// Node represents a Discord attachment URL and Size
type Node struct {
	NId   int64  // not used in ddrv package itself but for data providers
	URL   string `json:"url"`  // URL where the data is stored
	Size  int    `json:"size"` // Size of the data
	Start int64  // Start position of the data in the overall data sequence
	End   int64  // End position of the data in the overall data sequence
	MId   int64  `json:"mid"` // Node message id
	Ex    int    `json:"ex"`  // Node link expiry time
	Is    int    `json:"is"`  // Node link issued time
	Hm    string `json:"hm"`  // Node link signature
}

// Message represents a Discord message and contains attachments (files uploaded within the message).
type Message struct {
	Id          string `json:"id"`
	Attachments []Node `json:"attachments"`
}

const (
	TokenBot = iota
	TokenUser
	TokenUserNitro
	TokenUserNitroBasic
)
