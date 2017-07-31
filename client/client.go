package client

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

const (
	scheme       = "hfs://"
	secureScheme = "hfss://"
)

var (
	ErrInvalid = errors.New("invalid argument")
)

type hfsFile struct {
	addr string
}

func (c *hfsFile) secure() bool {
	return isSecure(c.addr)
}

func (c *hfsFile) Read(p []byte) (int, error) {

	// read file from remote address
	// using http get
	body := new(bytes.Buffer)
	req, err := http.NewRequest("GET", c.addr, body)
	if err != nil {
		return 0, err
	}
	req.Header.Add("Accept-Encoding", "gzip")

	client := createClient(c.secure())

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// read from compressed response
	buff := bytes.NewBuffer(p)
	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return 0, err
	}

	n, err := io.Copy(buff, gzipReader)

	return int(n), err
}

func (c *hfsFile) Write(p []byte) (int, error) {

	// request content must be compressed
	body := new(bytes.Buffer)
	gzipWritter := gzip.NewWriter(body)
	defer gzipWritter.Close()

	n, err := gzipWritter.Write(p)
	if err != nil {
		return 0, err
	}

	log.Println(body.String())

	// send file into remote address
	// using http upload
	req, err := http.NewRequest("POST", c.addr, body)
	if err != nil {
		return 0, err
	}
	req.Header.Add("Content-Encoding", "gzip")

	client := createClient(c.secure())

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// ensure response body read before closing
	ioutil.ReadAll(resp.Body)

	return int(n), err
}

func (c *hfsFile) Close() error {
	if c == nil {
		return ErrInvalid
	}

	return nil
}

// Open a file on remote address
func Open(addr string) (io.Reader, error) {
	return &hfsFile{addr: convertAddress(addr)}, nil
}

// Create a file on remote address
func Create(addr string) (io.Writer, error) {
	return &hfsFile{addr: convertAddress(addr)}, nil
}

// Remove delete file on remote address
func Remove(addr string) error {

	// remove file from remote address
	// using http delete
	body := new(bytes.Buffer)
	req, err := http.NewRequest("DELETE", addr, body)
	if err != nil {
		return err
	}

	client := createClient(isSecure(addr))
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	ioutil.ReadAll(resp.Body)

	return nil
}

// convertAddress converts hfs address to http equivalent
func convertAddress(addr string) string {
	if strings.HasPrefix(addr, scheme) {
		return strings.Replace(addr, scheme, "http://", 1)
	}

	if strings.HasPrefix(addr, secureScheme) {
		return strings.Replace(addr, secureScheme, "https://", 1)
	}

	// add additional scheme
	return "http://" + addr
}

func createClient(secure bool) *http.Client {
	if !secure {
		return http.DefaultClient
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	return &http.Client{Transport: transport}
}

func isSecure(addr string) bool {
	return strings.HasPrefix(addr, "https://")
}
