package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq" // Import the PostgreSQL driver
	zl "github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/forscht/ddrv/pkg/ddrv"
)

func main() {
	// Define flags with multi-line descriptions
	tokens := flag.String("tokens", "", "Discord bot/user token. This token is used to authenticate your bot with Discord's API.")
	tokenType := flag.String("token-type", "bot", "Determines the type of token used to authenticate with Discord's API, Possible values : ['bot','user']")
	dbURL := flag.String("db-url", "", "Postgres database url, you used for ddrv1")

	// Parse the flags
	flag.Parse()

	// Check if all flags are provided
	if *tokens == "" || *tokenType == "" || *dbURL == "" {
		fmt.Println("Error: All flags --tokens and --db-url must be provided")
		flag.Usage()
		os.Exit(1)
	}

	// Setup logger
	log.Logger = zl.New(zl.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
	zl.SetGlobalLevel(zl.DebugLevel)

	// Connect with postgres
	db, err := sql.Open("postgres", *dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("could not open postgres connection")
	}
	// Ping the database to ensure connectivity
	if err = db.Ping(); err != nil {
		log.Fatal().Err(err).Msg("postgres db ping failed")
	}
	log.Info().Msgf("connected with postgres")

	_, _ = db.Exec(`ALTER TABLE node ADD COLUMN mid BIGINT, ADD COLUMN ex INT, ADD COLUMN "is" INT, ADD COLUMN hm VARCHAR(255);`)
	log.Info().Msg("Added columns ex, is, hm and mid to table node")

	// Here we remove query parameters from attachment urls
	if _, err = db.Exec("UPDATE node SET url = split_part(url, '?', 1) WHERE mid IS NULL;"); err != nil {
		log.Fatal().Err(err).Msg("failed to clean urls on table 'node'")
	}
	log.Info().Msg("cleaned up urls on table node")

	// Prepare ddrv driver
	cfg := ddrv.Config{Tokens: []string{*tokens}, Channels: []string{"1"}}
	if *tokenType == "user" {
		cfg.TokenType = 1
	}
	driver, err := ddrv.New(&cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init ddrv driver")
	}

	// Define your slice to hold the channelIds
	var channelIds []string

	log.Info().Msg("creating index on 'url' column")
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_node_url ON node(url)")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create index on 'url'")
	}

	log.Info().Msg("starting migration from ddrv1 to ddrv2")
	// Perform the query
	rows, err := db.Query("SELECT DISTINCT split_part(url, '/', 5) AS channelId FROM node WHERE mid IS NULL;")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve unique channelIds from db")
	}
	defer rows.Close()

	// Iterate over the rows
	for rows.Next() {
		var channelId string
		if err = rows.Scan(&channelId); err != nil {
			log.Fatal().Err(err).Msg("failed to scan channelIds from rows")
		}
		channelIds = append(channelIds, channelId)
	}
	// Check for errors from iterating over rows
	if err = rows.Err(); err != nil {
		log.Fatal().Err(err).Msg("channelIds rows error")
	}

	log.Info().Msgf("retrieved %d channels from database", len(channelIds))
	for _, channelId := range channelIds {
		log.Info().Str("channel", channelId).Msg("processing migration")
		var messages []ddrv.Message
		var lastMessageId int64
		lastMessageId = 0
		for {
			// Reset the slice
			messages = messages[:0]
			// Fetch messages from discord
			if err = driver.Rest.GetMessages(channelId, lastMessageId, "before", &messages); err != nil {
				log.Fatal().Err(err).Str("channel", channelId).Msg("failed to fetch messages from discord")
			}
			// Exit the inner loop if no messages are left
			if len(messages) == 0 {
				break
			}
			for _, message := range messages {
				if len(message.Attachments) == 0 {
					continue
				}
				att := message.Attachments[0]
				url, ex, is, hm := ddrv.DecodeAttachmentURL(att.URL)
				if _, err = db.Exec(`UPDATE node SET ex=$1, "is"=$2, hm=$3, mid=$4 WHERE url=$5`, ex, is, hm, message.Id, url); err != nil {
					log.Fatal().Err(err).Int("ex", ex).Int("is", is).Str("hm", hm).
						Str("mid", message.Id).Str("parsed_url", url).Str("original_url", att.URL).
						Msg("failed to update attachment record")
				}
			}
			lastMessageId = stoi64(messages[len(messages)-1].Id)
		}
	}

	if _, err = db.Exec("DROP INDEX IF EXISTS idx_node_url"); err != nil {
		log.Fatal().Err(err).Str("idx", "idx_node_url").Msg("failed to drop index")
	}

	log.Info().Msg("ddrv2 migration complete")
}

func stoi64(str string) int64 {
	num, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		log.Fatal().Msgf("failed to convert string %s to int64", str)
	}
	return num
}
