package main

// Option provide customizable app settings
type Option struct {
	// Root folder
	Root string `yaml:"root"`

	// Enable conversion
	Compression bool `yaml:"compression"`

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
