package ddrv

import (
	"log"
	"net/url"
	"regexp"
	"strconv"
)

// This pattern matches the entire 'https://cdn.discordapp.com/attachments/' part
// and then captures a sequence of digits
var discordCDNRe = regexp.MustCompile(`https://cdn\.discordapp\.com/attachments/(\d+)/`)

// DecodeAttachmentURL parses the input URL and extracts the query parameters.
// It returns the cleaned URL, `ex` and `is` as integers, `hm` as a string, and an error if any.
func DecodeAttachmentURL(inputURL string) (string, int, int, string) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		log.Fatalf("DecodeAttachmentURL : failed to parse attachmentURL : URL -> %s", inputURL)
	}

	// Extract query parameters
	queryParams := parsedURL.Query()

	// Convert base16 (hexadecimal) values to int
	ex64, err := strconv.ParseInt(queryParams.Get("ex"), 16, 32)
	if err != nil {
		log.Fatalf("failed to convert ex to int : ex -> %s", queryParams.Get("ex"))
	}
	ex := int(ex64)

	is64, err := strconv.ParseInt(queryParams.Get("is"), 16, 32)
	if err != nil {
		log.Fatalf("failed to convert ex to int : is -> %s", queryParams.Get("is"))
	}
	is := int(is64)

	// Extract `hm` as a string
	hm := queryParams.Get("hm")

	// Clean URL
	cleanedURL := parsedURL.Scheme + "://" + parsedURL.Host + parsedURL.Path

	return cleanedURL, ex, is, hm
}

// EncodeAttachmentURL takes a base URL, `ex`, `is`, and `hm` as inputs, and returns the modified URL.
func EncodeAttachmentURL(baseURL string, ex int, is int, hm string) string {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		log.Fatalf("EncodeAttachmentURL : failed to parse attachmentURL : URL -> %s", baseURL)
	}

	// Convert int values to base16 (hexadecimal)
	exHex := strconv.FormatInt(int64(ex), 16)
	isHex := strconv.FormatInt(int64(is), 16)

	// Set query parameters
	queryParams := url.Values{}
	queryParams.Set("ex", exHex)
	queryParams.Set("is", isHex)
	queryParams.Set("hm", hm)

	// Construct the encoded URL
	parsedURL.RawQuery = queryParams.Encode()
	encodedURL := parsedURL.String()

	return encodedURL
}

func extractChannelId(url string) string {
	// Find the first match and extract the captured group
	matches := discordCDNRe.FindStringSubmatch(url)
	if len(matches) < 2 {
		log.Fatalf("extractChannelId : failed to extract channelId : URL -> %s", url)
	}

	// The channelId should be the second last part of the URL
	return matches[1]
}
