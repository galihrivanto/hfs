package server

import (
	"compress/gzip"
	"compress/zlib"
	"container/list"
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const fsMaxBufSize = 4096

// HfsServer define http file sharing server
type HfsServer struct {
	c      context.Context
	cancel context.CancelFunc
	server *http.Server
	option *Option
}

// Start running the server
func (s *HfsServer) Start() {

	server := http.Server{
		Handler: http.HandlerFunc(s.handleFile),
		Addr:    s.option.ServeAddress,
	}

	go func() {
		if s.option.SslCert != "" && s.option.SslKey != "" {
			log.Fatal(server.ListenAndServeTLS(s.option.SslCert, s.option.SslKey))
		} else {
			log.Fatal(server.ListenAndServe())
		}
	}()

	log.Printf("server running on %s \n", s.option.ServeAddress)

	sink := waitForSignals()
	defer close(sink)

	select {
	case <-s.c.Done():
		break
	case <-sink:
		break
	}

	log.Println("shutting down")
	server.Shutdown(s.c)
}

// Stop shutdown http file sharing server
func (s *HfsServer) Stop() {
	s.cancel()
}

// New create server instance with default option
func New() *HfsServer {
	return NewWithOption(nil)
}

// NewWithOption create new service instance
func NewWithOption(option *Option) *HfsServer {
	if option == nil {
		option = &Option{}
	}

	if option.Root == "" {
		option.Root = "."
	}

	if option.ServeAddress == "" {
		option.ServeAddress = ":3030"
	}

	c, cancel := context.WithCancel(context.Background())

	return &HfsServer{
		c:      c,
		cancel: cancel,
		option: option,
	}
}

func (s *HfsServer) handleFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", s.option.AppName)

	filepath := path.Join((s.option.Root), path.Clean(r.URL.Path))

	switch r.Method {
	case "GET":
		s.serveFile(filepath, w, r)
		break
	case "POST":
		s.handleUpload(filepath, w, r)
		break
	case "DELETE":
		s.handleRemove(filepath, w, r)
		break
	}

	log.Printf("\"%s %s %s\" \"%s\" \"%s\"\n",
		r.Method,
		r.URL.String(),
		r.Proto,
		r.Referer(),
		r.UserAgent())
}

func (s *HfsServer) handleUpload(filepath string, w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	// ensure target directory exist
	dir := path.Dir(filepath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	f, err := os.Create(filepath)
	if err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer f.Close()

	switch r.Header.Get("Content-Encoding") {
	case "gzip":

		gzipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "Internal Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		defer gzipReader.Close()

		if _, err := io.Copy(f, gzipReader); err != nil {
			http.Error(w, "Internal Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}

		break

	case "deflate":
		zlibReader, err := zlib.NewReader(r.Body)
		if err != nil {
			http.Error(w, "Internal Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		defer zlibReader.Close()

		if _, err := io.Copy(f, zlibReader); err != nil {
			http.Error(w, "Internal Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}

		break
	default:

		if _, err := io.Copy(f, r.Body); err != nil {
			http.Error(w, "Internal Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}

		break

	}

}

func (s *HfsServer) handleRemove(filepath string, w http.ResponseWriter, r *http.Request) {

	// open file handle
	f, err := os.Open(filepath)
	if err != nil {
		http.Error(w, "Not Found: Error while opening file", http.StatusNotFound)
		return
	}
	defer f.Close()

	// ensure opened file handle is a file
	statinfo, err := f.Stat()
	if err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// if directory
	if statinfo.IsDir() {
		http.Error(w, "Not Allowed: Delete directory is forbidden", http.StatusForbidden)
		return
	}

	// if socket mode, forbid!
	if (statinfo.Mode() &^ 07777) == os.ModeSocket {
		http.Error(w, "Not Allowed: Access to this resource is not allowed", http.StatusForbidden)
		return
	}

	if err := os.Remove(filepath); err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

func (s *HfsServer) serveFile(filepath string, w http.ResponseWriter, r *http.Request) {

	// open file handle
	f, err := os.Open(filepath)
	if err != nil {
		http.Error(w, "Not Found: Error while opening file", http.StatusNotFound)
		return
	}
	defer f.Close()

	// ensure opened file handle is a file
	statinfo, err := f.Stat()
	if err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	// if directory
	if statinfo.IsDir() {

		// if directory listing is not allowed
		if !s.option.DirListing {
			http.Error(w, "Not Allowed: Directory listing is forbidden", http.StatusForbidden)
			return
		}

		// handle directory listing
		s.handleDirectory(f, w, r)
		return
	}

	// if socket mode, forbid!
	if (statinfo.Mode() &^ 07777) == os.ModeSocket {
		http.Error(w, "Not Allowed: Access to this resource is not allowed", http.StatusForbidden)
		return
	}

	// Manages If-Modified-Since and add Last-Modified
	if t, err := time.Parse(http.TimeFormat, r.Header.Get("If-Modified-Since")); err == nil && statinfo.ModTime().Unix() <= t.Unix() {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Last-Modified", statinfo.ModTime().Format(http.TimeFormat))

	// Content-Type handling
	query, err := url.ParseQuery(r.URL.RawQuery)

	if err == nil && len(query["dl"]) > 0 { // The user explicitedly wanted to download the file (Dropbox style!)
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		// Fetching file's mimetype and giving it to the browser
		if mimetype := mime.TypeByExtension(path.Ext(filepath)); mimetype != "" {
			w.Header().Set("Content-Type", mimetype)
		} else {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
	}

	// Manage Content-Range (TODO: Manage end byte and multiple Content-Range)
	if r.Header.Get("Range") != "" {
		startByte := parseRange(r.Header.Get("Range"))

		if startByte < statinfo.Size() {
			f.Seek(startByte, 0)
		} else {
			startByte = 0
		}

		w.Header().Set("Content-Range",
			fmt.Sprintf("bytes %d-%d/%d", startByte, statinfo.Size()-1, statinfo.Size()))
	}

	// Manage gzip/zlib compression
	outputWriter := w.(io.Writer)
	isCompressedReply := false

	if r.Header.Get("Accept-Encoding") != "" {
		encodings := parseCSV(r.Header.Get("Accept-Encoding"))

		for _, val := range encodings {
			if val == "gzip" {

				w.Header().Set("Content-Encoding", "gzip")
				outputWriter = gzip.NewWriter(w)
				isCompressedReply = true
				break

			} else if val == "deflate" {

				w.Header().Set("Content-Encoding", "deflate")
				outputWriter = zlib.NewWriter(w)
				isCompressedReply = true

				break
			}
		}
	}

	if !isCompressedReply {
		// Add Content-Length
		w.Header().Set("Content-Length", strconv.FormatInt(statinfo.Size(), 10))
	}

	// Stream data out !
	buf := make([]byte, min(fsMaxBufSize, statinfo.Size()))
	n := 0
	for err == nil {
		n, err = f.Read(buf)
		outputWriter.Write(buf[0:n])
	}

	// Closes current compressors
	switch outputWriter.(type) {
	case *gzip.Writer:
		outputWriter.(*gzip.Writer).Close()
	case *zlib.Writer:
		outputWriter.(*zlib.Writer).Close()
	}
}

// Manages directory listings
type dirListing struct {
	Name     string
	Dirs     []string
	Files    []string
	ServerUA string
}

func copyToArray(src *list.List) []string {
	dst := make([]string, src.Len())

	i := 0
	for e := src.Front(); e != nil; e = e.Next() {
		dst[i] = e.Value.(string)
		i = i + 1
	}

	return dst
}

func (s *HfsServer) handleDirectory(f *os.File, w http.ResponseWriter, r *http.Request) {
	names, _ := f.Readdir(-1)

	// First, check if there is any index in this folder.
	for _, val := range names {
		if val.Name() == "index.html" {
			s.serveFile(path.Join(f.Name(), "index.html"), w, r)
			return
		}
	}

	// Otherwise, generate folder content.
	dirTmp := list.New()
	filesTmp := list.New()

	for _, val := range names {
		if val.Name()[0] == '.' {
			continue
		} // Remove hidden files from listing

		if val.IsDir() {
			dirTmp.PushBack(val.Name())
		} else {
			filesTmp.PushBack(val.Name())
		}
	}

	// And transfer the content to the final array structure
	dirs := copyToArray(dirTmp)
	files := copyToArray(filesTmp)

	tpl, err := template.New("tpl").Parse(tplDirListing)
	if err != nil {
		http.Error(w, "500 Internal Error : Error while generating directory listing.", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	data := dirListing{
		Name:     r.URL.Path,
		ServerUA: s.option.AppName,
		Dirs:     dirs,
		Files:    files,
	}

	err = tpl.Execute(w, data)
	if err != nil {
		log.Println(err)
	}
}

func parseCSV(data string) []string {

	splitted := strings.SplitN(data, ",", -1)
	tmp := make([]string, len(splitted))

	for i, val := range splitted {
		tmp[i] = strings.TrimSpace(val)
	}

	return tmp
}

func parseRange(data string) int64 {

	stop := (int64)(0)
	part := 0
	for i := 0; i < len(data) && part < 2; i = i + 1 {
		if part == 0 { // part = 0 <=> equal isn't met.
			if data[i] == '=' {
				part = 1
			}

			continue
		}

		if part == 1 { // part = 1 <=> we've met the equal, parse beginning
			if data[i] == ',' || data[i] == '-' {
				part = 2 // part = 2 <=> OK.
			} else {
				if 48 <= data[i] && data[i] <= 57 { // If it's a digit ...
					// ... convert the char to integer and add it!
					stop = (stop * 10) + (((int64)(data[i])) - 48)
				} else {
					part = 2 // Parsing error! No error needed : 0 = from start.
				}
			}
		}
	}

	return stop
}

func min(x int64, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

const tplDirListing = `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en">
<!-- Modified from lighttpd directory listing -->
<head>
<title>Index of {{.Name}}</title>
<style type="text/css">
a, a:active {text-decoration: none; color: blue;}
a:visited {color: #48468F;}
a:hover, a:focus {text-decoration: underline; color: red;}
body {background-color: #F5F5F5;}
h2 {margin-bottom: 12px;}
table {margin-left: 12px;}
th, td { font: 90% monospace; text-align: left;}
th { font-weight: bold; padding-right: 14px; padding-bottom: 3px;}
td {padding-right: 14px;}
td.s, th.s {text-align: right;}
div.list { background-color: white; border-top: 1px solid #646464; border-bottom: 1px solid #646464; padding-top: 10px; padding-bottom: 14px;}
div.foot { font: 90% monospace; color: #787878; padding-top: 4px;}
</style>
</head>
<body>
<h2>Index of {{.Name}}</h2>
<div class="list">
<table summary="Directory Listing" cellpadding="0" cellspacing="0">
<thead><tr><th class="n">Name</th><th class="t">Type</th><th class="dl">Options</th></tr></thead>
<tbody>
<tr><td class="n"><a href="../">Parent Directory</a>/</td><td class="t">Directory</td><td class="dl"></td></tr>
{{range .Dirs}}
<tr><td class="n"><a href="{{.}}/">{{.}}/</a></td><td class="t">Directory</td><td class="dl"></td></tr>
{{end}}
{{range .Files}}
<tr><td class="n"><a href="{{.}}">{{.}}</a></td><td class="t">&nbsp;</td><td class="dl"><a href="{{.}}?dl">Download</a></td></tr>
{{end}}
</tbody>
</table>
</div>
<div class="foot">{{.ServerUA}}</div>
</body>
</html>`
