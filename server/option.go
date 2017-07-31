package server

// Option provide customizable app settings
type Option struct {
	AppName string `yaml:"appname"`

	// Root folder
	Root string `yaml:"root"`

	// Allow Directory browsing and listing
	DirListing bool `yaml:"dirlisting"`

	// http serve address
	ServeAddress string `yaml:"address"`

	// Verbose output
	Verbose bool `yaml:"verbose"`

	// SSL Certificate and Key file pair
	SslCert string `yaml:"sslcert"`
	SslKey  string `yaml:"sslkey"`
}
