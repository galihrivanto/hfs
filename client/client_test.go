package client

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/galihrivanto/hfs/server"
)

func TestConvertAddress(t *testing.T) {
	testAddress := "hfs://localhost:3030/dir/file.ext"
	expect := "http://localhost:3030/dir/file.ext"

	res := convertAddress(testAddress)
	if expect != res {
		t.Errorf("expect %s but returned %s", expect, res)
	}

	testAddress = "hfss://localhost:3030/dir/secure_file.ext"
	expect = "https://localhost:3030/dir/secure_file.ext"

	res = convertAddress(testAddress)
	if expect != res {
		t.Errorf("expect %s but returned %s", expect, res)
	}
}

func TestActions(t *testing.T) {
	testpath := "hfs://localhost:3030/test.txt"
	testcontent := "test dummy content"

	// running server
	srv := server.NewWithOption(&server.Option{
		ServeAddress: ":3030",
		Root:         ".",
		Verbose:      true,
	})

	go srv.Start()
	defer srv.Stop()

	<-time.After(5 * time.Second)

	// create file
	fw, err := Create(testpath)
	if err != nil {
		t.Error(err)
		return
	}
	_, err = fw.Write([]byte(testcontent))
	if err != nil {
		t.Error(err)
		return
	}

	t.Log("write done!")

	// read file
	fr, err := Open(testpath)
	if err != nil {
		t.Error(err)
		return
	}

	buff := new(bytes.Buffer)
	_, err = io.Copy(buff, fr)
	if err != nil {
		t.Error(err)
		return
	}

	t.Log("read done!")

	if buff.String() != testcontent {
		t.Error("read content invalid")
		return
	}

	if err := Remove(testpath); err != nil {
		t.Error(err)
		return
	}
}
