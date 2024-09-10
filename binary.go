// Протокол общения с воркерами. Очень простой, поддерживает два
// базовых типа данных:
// 1) string (или []byte)
// 2) uint64
// 
// Все данные записываются/читаются в little endian. Тип данных в самом поле не
// указывается. Порядок записи и чтения полей должен повторяться, т.е. порядок
// полей имеет значение.
// 
// Строка записывается так: сначала uint64 с количеством байт в строке, потом
// сама строка:
// [len(str)][str]
// 
// Массивы записываются так: сначала uint64 с количеством элементов, потом сами
// элементы подряд:
// [len(arr)][element1][element2][...]
// 
// Карты/словари записываются так: сначала uint64 с количеством элементов,
// потом пара: ключ элемента и сам элемент:
// [len(map)][key1][value1][key2][value2][...].
//
// Реализованы только нужные типы данных и операции над ними.
package corerunner

import (
	"encoding/binary"
	"io"
)

// HTTP-запрос, который Go процесс передает в воркер в бинарном виде.
type HTTPRequest struct {
	Method string
	URL string
	Headers map[string]string
	Body []byte
	Files map[string]*File
	Form map[string]string
}

// Write сериализует HTTP-запрос с записью в указанный io.Writer.
func (hr *HTTPRequest) Write(w io.Writer) error {
	err := writeString(w, hr.Method)
	if err != nil {
		return err
	}
	err = writeString(w, hr.URL)
	if err != nil {
		return err
	}
	err = writeStringMap(w, hr.Headers)
	if err != nil {
		return err
	}
	err = writeBytes(w, hr.Body)
	if err != nil {
		return err
	}
	err = writeFileMap(w, hr.Files)
	if err != nil {
		return err
	}
	return writeStringMap(w, hr.Form)
}

// Parse считывает HTTP-запрос из указанного io.Reader.
func (hr *HTTPRequest) Parse(r io.Reader) error {
	method, err := parseString(r)
	if err != nil {
		return err
	}
	hr.Method = method
	url, err := parseString(r)
	if err != nil {
		return err
	}
	hr.URL = url
	headers, err := parseStringMap(r)
	if err != nil {
		return err
	}
	hr.Headers = headers
	body, err := parseBytes(r)
	if err != nil {
		return err
	}
	hr.Body = body
	files, err := parseFileMap(r)
	if err != nil {
		return err
	}
	hr.Files = files
	form, err := parseStringMap(r)
	if err != nil {
		return err
	}
	hr.Form = form
	return nil
}

// HTTP-файл, который Go процесс передает в воркер в бинарном виде.
type File struct {
	Filename string
	TmpPath string
	Size uint64
}

// Write сериализует HTTP-файл с записью в указанный io.Writer.
func (f *File) Write(w io.Writer) error {
	err := writeString(w, f.Filename)
	if err != nil {
		return err
	}
	err = writeString(w, f.TmpPath)
	if err != nil {
		return err
	}
	return writeUint64(w, f.Size)
}

// Parse считывает HTTP-файл из указанного io.Reader.
func (f *File) Parse(r io.Reader) error {
	filename, err := parseString(r)
	if err != nil {
		return err
	}
	f.Filename = filename
	tmpPath, err := parseString(r)
	if err != nil {
		return err
	}
	f.TmpPath = tmpPath
	size, err := parseUint64(r)
	if err != nil {
		return err
	}
	f.Size = size
	return nil
}

// HTTP-ответ, возвращаемый воркером в бинарном виде в Go-процесс.
type HTTPResponse struct {
	StatusCode uint64
	Headers map[string]string
	Body []byte
}

// Write сериализует HTTP-файл с записью в указанный io.Writer.
func (hr *HTTPResponse) Write(w io.Writer) error {
	err := writeUint64(w, hr.StatusCode)
	if err != nil {
		return err
	}
	err = writeStringMap(w, hr.Headers)
	if err != nil {
		return err
	}
	return writeBytes(w, hr.Body)
}

// Parse считывает HTTP-ответ из указанного io.Reader.
func (hr *HTTPResponse) Parse(r io.Reader) error {
	code, err := parseUint64(r)
	if err != nil {
		return err
	}
	hr.StatusCode = code
	headers, err := parseStringMap(r)
	if err != nil {
		return err
	}
	hr.Headers = headers
	body, err := parseBytes(r)
	if err != nil {
		return err
	}
	hr.Body = body
	return nil
}

// Задача, отправляемая в бинарном виде в воркер для обработки. Задача при
// выполнении возвращает результат JobResponse.
type JobRequest struct {
	Name string
	Payload []byte
	Timeout uint64
}

// Write сериализует задачу с записью в указанный io.Writer.
func (jr *JobRequest) Write(w io.Writer) error {
	err := writeString(w, jr.Name)
	if err != nil {
		return err
	}
	err = writeBytes(w, jr.Payload)
	if err != nil {
		return err
	}
	return writeUint64(w, jr.Timeout)
}

// Parse считывает задачу из указанного io.Reader.
func (jr *JobRequest) Parse(r io.Reader) error {
	name, err := parseString(r)
	if err != nil {
		return err
	}
	jr.Name = name
	payload, err := parseBytes(r)
	if err != nil {
		return err
	}
	jr.Payload = payload
	timeout, err := parseUint64(r)
	if err != nil {
		return err
	}
	jr.Timeout = timeout
	return nil
}

// Ответ из воркера после обработки JobRequest.
type JobResponse struct {
	Payload []byte
}

// Write сериализует ответ задачи с записью в указанный io.Writer.
func (jr *JobResponse) Write(w io.Writer) error {
	return writeBytes(w, jr.Payload)
}

// Parse считывает ответ задачи из указанного io.Reader.
func (jr *JobResponse) Parse(r io.Reader) error {
	payload, err := parseBytes(r)
	if err != nil {
		return err
	}
	jr.Payload = payload
	return nil
}

func writeString(w io.Writer, val string) error {
	return writeBytes(w, []byte(val))
}

func parseString(r io.Reader) (string, error) {
	res, err := parseBytes(r)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func writeUint64(w io.Writer, val uint64) error {
	return binary.Write(w, binary.LittleEndian, val)
}

func parseUint64(r io.Reader) (uint64, error) {
	var res uint64
	err := binary.Read(r, binary.LittleEndian, &res)
	if err != nil {
		return 0, err
	}
	return res, nil
}

func writeBytes(w io.Writer, val []byte) error {
	err := writeUint64(w, uint64(len(val)))
	if err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, val)
}

func parseBytes(r io.Reader) ([]byte, error) {
	var l uint64
	err := binary.Read(r, binary.LittleEndian, &l)
	if err != nil {
		return nil, err
	}
	res := make([]byte, l)
	err = binary.Read(r, binary.LittleEndian, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func writeStringMap(w io.Writer, val map[string]string) error {
	err := writeUint64(w, uint64(len(val)))
	if err != nil {
		return err
	}
	for k, v := range val {
		err = writeString(w, k)
		if err != nil {
			return err
		}
		err = writeString(w, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func parseStringMap(r io.Reader) (map[string]string, error) {
	l, err := parseUint64(r)
	if err != nil {
		return nil, err
	}
	res := make(map[string]string, l)
	for i := uint64(0); i < l; i++ {
		k, err := parseString(r)
		if err != nil {
			return nil, err
		}
		v, err := parseString(r)
		if err != nil {
			return nil, err
		}
		res[k] = v
	}
	return res, nil
}

func writeFileMap(w io.Writer, val map[string]*File) error {
	err := writeUint64(w, uint64(len(val)))
	if err != nil {
		return err
	}
	for k, v := range val {
		err = writeString(w, k)
		if err != nil {
			return err
		}
		err = v.Write(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func parseFileMap(r io.Reader) (map[string]*File, error) {
	l, err := parseUint64(r)
	if err != nil {
		return nil, err
	}
	res := make(map[string]*File, l)
	for i := uint64(0); i < l; i++ {
		k, err := parseString(r)
		if err != nil {
			return nil, err
		}
		f := &File{}
		err = f.Parse(r)
		if err != nil {
			return nil, err
		}
		res[k] = f
	}
	return res, nil
}
