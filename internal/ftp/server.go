package ftp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/fclairamb/ftpserverlib"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"

	"github.com/forscht/ddrv/internal/filesystem"
	"github.com/forscht/ddrv/pkg/ddrv"
)

const IPResolveURL = "https://ipinfo.io/ip"

// Define custom error messages
var (
	ErrNoTLS                 = errors.New("TLS is not configured")    // Error for missing TLS configuration
	ErrBadUserNameOrPassword = errors.New("bad username or password") // Error for failed authentication
)

type Config struct {
	Addr       string `mapstructure:"addr"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	PortRange  string `mapstructure:"port_range"`
	AsyncWrite bool   `mapstructure:"async_write"`
}

func Serv(drvr *ddrv.Driver, cfg *Config) error {
	// If Addr not provided, do not start FTP server
	if cfg.Addr == "" {
		return nil
	}
	var portRange *ftpserver.PortRange
	if cfg.PortRange != "" {
		portRange = &ftpserver.PortRange{}
		if _, err := fmt.Sscanf(cfg.PortRange, "%d-%d", &portRange.Start, &portRange.End); err != nil {
			log.Fatal().Str("c", "ftpserver").Int("portstart", portRange.Start).
				Int("portend", portRange.End).Err(err).Msg("bad port range")
		}
	}
	fs := filesystem.New(drvr, cfg.AsyncWrite)
	driver := &Driver{
		Fs:       fs,           // The file system to serve over FTP
		username: cfg.Username, // Username for authentication
		password: cfg.Password, // Password for authentication
		Settings: &ftpserver.Settings{
			ListenAddr:          cfg.Addr,                     // The network address to listen on
			DefaultTransferType: ftpserver.TransferTypeBinary, // Default to binary transfer mode
			// Stooopid FTP thinks connection is idle, even when file transfer is going on.
			// Default is 900 seconds so, after which the server will drop the connection
			// Increased it to 24 hours to allow big file transfers
			IdleTimeout: 86400, // 24 hour
		},
	}

	// Enable PASV mode of portRange is supplied
	if portRange != nil {
		// Range of ports for passive FTP connections
		driver.Settings.PassiveTransferPortRange = portRange
		// Function to resolve the static IP of the server
		driver.Settings.PublicIPResolver = func(context ftpserver.ClientContext) (string, error) {
			resp, err := http.Get(IPResolveURL) // Fetch static IP
			if err != nil {
				return "", err
			}
			ip, err := io.ReadAll(resp.Body)
			if err != nil {
				return "", err
			}
			return string(ip), nil
		}
	}

	// Instantiate the FTP server with the driver and return a pointer to it
	server := ftpserver.NewFtpServer(driver)
	log.Info().Str("c", "ftp").Str("addr", cfg.Addr).Msg("starting ftp server")

	return server.ListenAndServe()
}

// Driver is the FTP server driver implementation.
type Driver struct {
	Fs       afero.Fs            // The file system to serve over FTP
	Debug    bool                // Debug mode flag
	Settings *ftpserver.Settings // The FTP server settings
	username string              // Username for authentication
	password string              // Password for authentication
}

// ClientConnected is called when a client is connected to the FTP server.
func (d *Driver) ClientConnected(cc ftpserver.ClientContext) (string, error) {
	log.Info().Str("c", "ftpserver").Str("addr", cc.RemoteAddr().String()).
		Str("client", cc.GetClientVersion()).Uint32("id", cc.ID()).Msg("client connected")
	return "DDrv FTP Server", nil // Return a welcome message
}

// ClientDisconnected is called when a client is disconnected from the FTP server.
func (d *Driver) ClientDisconnected(cc ftpserver.ClientContext) {
	log.Info().Str("c", "ftpserver").Str("addr", cc.RemoteAddr().String()).
		Str("client", cc.GetClientVersion()).Uint32("id", cc.ID()).Msg("client disconnected")
}

// AuthUser authenticates a user during the FTP server login process.
func (d *Driver) AuthUser(cc ftpserver.ClientContext, user, pass string) (ftpserver.ClientDriver, error) {
	// If authentication is required, check the provided username and password against the expected values
	if d.username != "" && d.username != user || d.password != "" && d.password != pass {
		log.Info().Str("c", "ftpserver").Str("addr", cc.RemoteAddr().String()).Uint32("id", cc.ID()).
			Str("user", user).Str("pass", pass).Err(ErrBadUserNameOrPassword).Msg("authentication failed")
		return nil, ErrBadUserNameOrPassword // If either check fails, return an authentication error
	}
	return d.Fs, nil // If the checks pass or authentication is not required, proceed with the provided file system
}

// GetSettings returns the FTP server settings.
func (d *Driver) GetSettings() (*ftpserver.Settings, error) { return d.Settings, nil }

// GetTLSConfig returns the TLS configuration for the FTP server.
func (d *Driver) GetTLSConfig() (*tls.Config, error) { return nil, ErrNoTLS } // The server does not support TLS, so return a "no TLS" error
