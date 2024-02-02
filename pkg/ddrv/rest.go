package ddrv

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	baseURL    = "https://discord.com/api/v10"
	UserAgent  = "PostmanRuntime/7.35.0"
	ReqTimeout = 60 * time.Second
)

type Rest struct {
	channels     []string
	nitro        bool
	limiter      *Limiter
	client       *http.Client
	tokens       []string
	mutex        *sync.Mutex
	lastChIdx    int
	lastTokenIdx int
	chunkSize    int
}

func NewRest(tokens []string, channels []string, chunkSize int, nitro bool) *Rest {
	return &Rest{
		client:       &http.Client{Timeout: ReqTimeout},
		channels:     channels,
		nitro:        nitro,
		limiter:      NewLimiter(),
		tokens:       tokens,
		mutex:        &sync.Mutex{},
		lastTokenIdx: 0,
		lastChIdx:    0,
		chunkSize:    chunkSize,
	}
}

// token returns the next token in a round-robin fashion.
func (r *Rest) token() string {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	token := r.tokens[r.lastTokenIdx]
	r.lastTokenIdx = (r.lastTokenIdx + 1) % len(r.tokens)
	return token
}

// channel returns the next channel in a round-robin fashion.
func (r *Rest) channel() string {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	channel := r.channels[r.lastChIdx]
	r.lastChIdx = (r.lastChIdx + 1) % len(r.channels)
	return channel
}

func (r *Rest) doReq(bucketId string, req *http.Request, retry bool) (*http.Response, error) {

	token := r.token()
	bucketId = token + bucketId
	req.Header.Add("User-Agent", UserAgent)
	req.Header.Add("Authorization", token)

	r.limiter.Acquire(bucketId)

	// Here make HTTP call
	resp, err := r.client.Do(req)
	// Release lock
	if resp != nil && resp.Header != nil {
		r.limiter.Release(bucketId, resp.Header)
		if retry &&
			(resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode > http.StatusInternalServerError) {
			return r.doReq(bucketId, req, retry)
		}
	} else {
		r.limiter.Release(bucketId, nil)
	}
	return resp, err
}

func (r *Rest) GetMessages(channelId string, messageId int64, query string, messages *[]Message) error {
	var path string
	if messageId != 0 && query != "" {
		path = fmt.Sprintf("/channels/%s/messages?limit=100&%s=%d", channelId, query, messageId)
	} else {
		path = fmt.Sprintf("/channels/%s/messages?limit=100", channelId)
	}
	bucketId := fmt.Sprintf("/%s/messages", channelId)

	// Create request
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := r.doReq(bucketId, req, true)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rest getmessages: expected status code %d - received %d", http.StatusOK, resp.StatusCode)
	}
	// read and parse the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(respBody, messages); err != nil {
		return err
	}
	return nil
}

// CreateAttachment uploads a file to the Discord channel using the webhook.
func (r *Rest) CreateAttachment(reader io.Reader) (*Node, error) {
	// If nitro enabled, use another method to create the attachment
	if r.nitro {
		return r.CreateAttachmentNitro(reader)
	}
	channelId := r.channel()
	path := fmt.Sprintf("/channels/%s/messages", channelId)
	bucketId := fmt.Sprintf("/%s/messages", channelId)

	// Prepare request
	contentType, body := mbody(reader)
	req, err := http.NewRequest(http.MethodPost, baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", contentType)

	// Here make HTTP call
	resp, err := r.doReq(bucketId, req, false)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("create attachment : expected status code %d but recevied %d", http.StatusOK, resp.StatusCode)
	}
	// read and parse the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var m Message
	if err = json.Unmarshal(respBody, &m); err != nil {
		return nil, err
	}
	// clean url and extract ex,is and hm
	att := m.Attachments[0]
	att.URL, att.Ex, att.Is, att.Hm = DecodeAttachmentURL(att.URL)
	att.MId, _ = strconv.ParseInt(m.Id, 10, 64)
	// Return the first attachment from the response
	return &att, nil
}

type AttachmentResp struct {
	Attachments []struct {
		UploadUrl      string `json:"upload_url"`
		UploadFileName string `json:"upload_filename"`
	} `json:"attachments"`
}

func (r *Rest) CreateAttachmentNitro(reader io.Reader) (*Node, error) {
	// 1. Request to get upload URL
	fname := uuid.New().String()
	channelId := r.channel()
	bucketId := fmt.Sprintf("/%s/messages", channelId)

	path := fmt.Sprintf("/channels/%s/attachments", channelId)
	body := fmt.Sprintf(`{"files":[{"filename":"%s","file_size": %d}]}`, fname, r.chunkSize)
	req, err := http.NewRequest(http.MethodPost, baseURL+path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := r.doReq(bucketId, req, true)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("create attachment : expected status code %d but recevied %d", http.StatusOK, resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var ar AttachmentResp
	if err = json.Unmarshal(respBody, &ar); err != nil {
		return nil, err
	}
	a := ar.Attachments[0]

	// 2. Request to upload binary data
	req, err = http.NewRequest(http.MethodPut, a.UploadUrl, reader)
	if err != nil {
		return nil, err
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to upload chunk to upload url : %v", err)
	}

	// 3. Request to create a message in channel
	path = fmt.Sprintf("/channels/%s/messages", channelId)
	body = fmt.Sprintf(`{"attachments":[{"id":"0","filename":"%s","uploaded_filename":"%s"}]}`, fname, a.UploadFileName)
	req, err = http.NewRequest(http.MethodPost, baseURL+path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err = r.doReq(bucketId, req, true)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("create attachment : expected status code %d but recevied %d", http.StatusOK, resp.StatusCode)
	}
	// read and parse the response body
	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var m Message
	if err = json.Unmarshal(respBody, &m); err != nil {
		return nil, err
	}
	// clean url and extract ex,is and hm
	node := m.Attachments[0]
	node.URL, node.Ex, node.Is, node.Hm = DecodeAttachmentURL(node.URL)
	node.MId, _ = strconv.ParseInt(m.Id, 10, 64)

	// Return the first attachment from the response
	return &node, nil
}

func (r *Rest) ReadAttachment(att *Node, start int, end int) (io.ReadCloser, error) {
	path := EncodeAttachmentURL(att.URL, att.Ex, att.Is, att.Hm)
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	// Set the Range header to specify the range of data to fetch
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > http.StatusInternalServerError {
		return r.ReadAttachment(att, start, end)
	}
	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("read attachment : expected code %d but received %d", http.StatusPartialContent, resp.StatusCode)
	}
	// Return the body of the response, which contains the requested data
	return resp.Body, nil
}

// mbody creates the multipart form-data body to upload a file to the Discord channel using the webhook.
func mbody(reader io.Reader) (string, io.Reader) {
	boundary := "disgosucks"
	// Set the content type including the boundary
	contentType := fmt.Sprintf("multipart/form-data; boundary=%s", boundary)

	CRLF := "\r\n"
	fname := uuid.New().String()

	// Assemble all the parts of the multipart form-data
	parts := []io.Reader{
		strings.NewReader("--" + boundary + CRLF),
		strings.NewReader(fmt.Sprintf(`Content-Disposition: form-data; name="%s"; filename="%s"`, fname, fname) + CRLF),
		strings.NewReader(fmt.Sprintf(`Content-Type: %s`, "application/octet-stream") + CRLF),
		strings.NewReader(CRLF),
		reader,
		strings.NewReader(CRLF),
		strings.NewReader("--" + boundary + "--" + CRLF),
	}

	// Return the content type and the combined reader of all parts
	return contentType, io.MultiReader(parts...)
}
