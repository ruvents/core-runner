package corerunner

import (
	"bytes"
	"testing"
	"time"
)

func TestStringSerialization(t *testing.T) {
	want := "hello!"
	buf := bytes.Buffer{}
	err := writeString(&buf, want)
	if err != nil {
		t.Fatalf("could not write a string into the buffer: %s", err)
	}
	r := bytes.NewReader(buf.Bytes())
	val, err := parseString(r)
	if err != nil {
		t.Fatalf("could not parse a string from the buffer: %s", err)
	}
	if val != want {
		t.Fatalf(
			"written and parsed string do not match: %s and %s",
			want,
			val,
		)
	}
}

func TestUint64Serialization(t *testing.T) {
	want := uint64(123456)
	buf := bytes.Buffer{}
	err := writeUint64(&buf, want)
	if err != nil {
		t.Fatalf("could not write bytes into the buffer: %s", err)
	}
	r := bytes.NewReader(buf.Bytes())
	val, err := parseUint64(r)
	if err != nil {
		t.Fatalf("could not parse int64 from the buffer: %s", err)
	}
	if val != want {
		t.Fatalf(
			"written and parsed int64 do not match: %d and %d",
			want,
			val,
		)
	}
}

func TestBytesSerialization(t *testing.T) {
	want := []byte{0x62, 0x79, 0x0, 0x21}
	buf := bytes.Buffer{}
	err := writeBytes(&buf, want)
	if err != nil {
		t.Fatalf("could not write bytes into the buffer: %s", err)
	}
	r := bytes.NewReader(buf.Bytes())
	val, err := parseBytes(r)
	if err != nil {
		t.Fatalf("could not parse bytes from the buffer: %s", err)
	}
	if !bytes.Equal(val, want) {
		t.Fatalf(
			"written and parsed bytes do not match: %s and %s",
			want,
			val,
		)
	}
}

func TestStringMapSerialization(t *testing.T) {
	want := make(map[string]string)
	want["foo"] = "bar"
	want["one"] = "two"
	buf := bytes.Buffer{}
	err := writeStringMap(&buf, want)
	if err != nil {
		t.Fatalf("could not write a string map into the buffer: %s", err)
	}
	r := bytes.NewReader(buf.Bytes())
	val, err := parseStringMap(r)
	if err != nil {
		t.Fatalf("could not parse a string map from the buffer: %s", err)
	}
	if !equalStringMaps(val, want) {
		t.Fatalf(
			"written and parsed string maps do not match: %s and %s",
			want,
			val,
		)
	}
}

func TestHTTPResponseSerialization(t *testing.T) {
	headers := make(map[string]string)
	headers["Authentication"] = "Bearer TOKEN!"
	headers["Content-Type"] = "application/json"
	want := HTTPResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       []byte{'h', 'e', 'l', 'l', 'o', '!'},
	}
	buf := bytes.Buffer{}
	err := want.Write(&buf)
	if err != nil {
		t.Fatalf("could not write an HTTP response: %s", err)
	}
	got := HTTPResponse{}
	err = got.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("could not parse an HTTP response: %s", err)
	}
	if got.StatusCode != want.StatusCode ||
		!equalStringMaps(got.Headers, want.Headers) ||
		!bytes.Equal(got.Body, want.Body) {
		t.Fatalf(
			"written and parsed HTTP responses do not match: %v and %v",
			want,
			got,
		)
	}
}

func TestFileMapSerialization(t *testing.T) {
	want := make(map[string]*File)
	want["foo"] = &File{TmpPath: "/tmp/1", Filename: "1", Size: 1}
	want["bar"] = &File{TmpPath: "/tmp/2", Filename: "2", Size: 2}
	buf := bytes.Buffer{}
	err := writeFileMap(&buf, want)
	if err != nil {
		t.Fatalf("could not write a file map into the buffer: %s", err)
	}
	r := bytes.NewReader(buf.Bytes())
	got, err := parseFileMap(r)
	if err != nil {
		t.Fatalf("could not parse a file map from the buffer: %s", err)
	}
	if !equalFileMaps(want, got) {
		t.Fatalf(
			"written and parsed file maps do not match: %v and %v",
			want,
			got,
		)
	}
}

func TestFileSerialization(t *testing.T) {
	want := File{
		Size:     123,
		TmpPath:  "/tmp/foobar.tmp",
		Filename: "foobar.tmp",
	}
	buf := bytes.Buffer{}
	err := want.Write(&buf)
	if err != nil {
		t.Fatalf("could not write a File: %s", err)
	}
	got := File{}
	err = got.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("could not parse a File: %s", err)
	}
	if got.Filename != want.Filename ||
		got.TmpPath != want.TmpPath ||
		got.Size != want.Size {
		t.Fatalf(
			"written and parsed Files do not match: %v and %v",
			want,
			got,
		)
	}
}

func TestHTTPRequestSerialization(t *testing.T) {
	headers := make(map[string]string)
	headers["Authentication"] = "Bearer TOKEN!"
	headers["Content-Type"] = "application/json"
	files := make(map[string]*File)
	files["foo"] = &File{TmpPath: "/tmp/1", Filename: "1", Size: 1}
	files["bar"] = &File{TmpPath: "/tmp/2", Filename: "2", Size: 2}
	form := make(map[string]string)
	form["form"] = "value"
	want := &HTTPRequest{
		Method:  "POST",
		URL:     "https://test.ru",
		Headers: headers,
		Files:   files,
		Form:    form,
	}
	buf := bytes.Buffer{}
	err := want.Write(&buf)
	if err != nil {
		t.Fatalf("could not write an HTTP request: %s", err)
	}
	got := HTTPRequest{}
	err = got.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("could not parse an HTTP request: %s", err)
	}
	if got.Method != want.Method ||
		got.URL != want.URL ||
		!bytes.Equal(got.Body, want.Body) ||
		!equalStringMaps(got.Headers, want.Headers) ||
		!equalFileMaps(got.Files, want.Files) ||
		!equalStringMaps(got.Form, want.Form) {
		t.Fatalf(
			"written and parsed HTTP requests do not match: %v and %v",
			want,
			got,
		)
	}
}

func TestJobRequestSerialization(t *testing.T) {
	want := &JobRequest{
		Name:    "testName",
		Payload: []byte("test payload!!"),
		Timeout: 123,
	}
	buf := bytes.Buffer{}
	err := want.Write(&buf)
	if err != nil {
		t.Fatalf("could not write a job request: %s", err)
	}
	got := JobRequest{}
	err = got.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("could not parse a job request: %s", err)
	}
	if got.Name != want.Name ||
		!bytes.Equal(got.Payload, want.Payload) ||
		got.Timeout != want.Timeout {
		t.Fatalf(
			"written and parsed job requests do not match: %v and %v",
			want,
			got,
		)
	}
}

func TestJobResponseSerialization(t *testing.T) {
	want := &JobResponse{
		Payload: []byte("test payload!!"),
	}
	buf := bytes.Buffer{}
	err := want.Write(&buf)
	if err != nil {
		t.Fatalf("could not write a job response: %s", err)
	}
	got := JobResponse{}
	err = got.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("could not parse a job response: %s", err)
	}
	if !bytes.Equal(got.Payload, want.Payload) {
		t.Fatalf(
			"written and parsed job responses do not match: %v and %v",
			want,
			got,
		)
	}
}

func TestPHPIntegration(t *testing.T) {
	headers := make(map[string]string)
	headers["Authentication"] = "Bearer TOKEN!"
	headers["Content-Type"] = "application/json"
	files := make(map[string]*File)
	files["foo"] = &File{TmpPath: "/tmp/1", Filename: "1", Size: 1}
	form := make(map[string]string)
	form["form"] = "value"
	req := &HTTPRequest{
		Method:  "POST",
		URL:     "https://test.ru",
		Body:    []byte("test"),
		Headers: headers,
		Files:   files,
		Form:    form,
	}
	buf := bytes.Buffer{}
	req.Write(&buf)

	ch := make(chan WorkerJob)
	resCh := make(chan WorkerResult)
	wrk := NewWorker(ch)
	err := wrk.Start([]string{"php", "./php/http.php"}, []string{})
	if err != nil {
		t.Fatal(err)
	}
	defer wrk.Stop()
	ch <- WorkerJob{data: buf.Bytes(), res: resCh, timeout: time.Second * 5}
	got := <-resCh

	rbuf := bytes.NewBuffer(got.Res)
	resp := HTTPResponse{}
	resp.Parse(rbuf)

	wantBody := `{"body":"test","files":{"foo":{"filename":"1","size":1,"tmpPath":"\/tmp\/1"}},"form":{"form":"value"}}`
	if resp.StatusCode != 200 ||
		!equalStringMaps(resp.Headers, headers) ||
		string(resp.Body) != wantBody {
		t.Fatalf("Некорректный ответ от PHP: %v", resp)
	}
}

func equalStringMaps(a map[string]string, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok || v != bv {
			return false
		}
	}
	return true
}

func equalFileMaps(a map[string]*File, b map[string]*File) bool {
	if len(a) != len(b) {
		return false
	}
	for k, a := range a {
		b, ok := b[k]
		if !ok {
			return false
		}
		if a.Filename != b.Filename ||
			a.TmpPath != b.TmpPath ||
			a.Size != b.Size {
			return false
		}
	}
	return true
}
