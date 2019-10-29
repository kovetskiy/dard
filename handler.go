package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/reconquest/karma-go"
)

type Handler struct {
	dir    string
	size   int64
	length int
}

func (handler *Handler) ServeHTTP(
	response http.ResponseWriter,
	request *http.Request,
) {
	if request.Method == "POST" {
		handler.upload(response, request)
	} else {
		handler.download(response, request)
	}
}

func (handler *Handler) upload(
	response http.ResponseWriter,
	request *http.Request,
) {
	err := request.ParseMultipartForm(handler.size)
	if err != nil {
		log.Println(karma.Format(
			err,
			"unable to parse form",
		))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	file, header, err := request.FormFile("file")
	if err != nil {
		if err == http.ErrMissingFile {
			response.WriteHeader(http.StatusBadRequest)
			return
		}

		internalError(response, karma.Format(
			err,
			"unable to get form file",
		))
		return
	}

	token, dir := handler.getTokenDir()

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		internalError(response, karma.Format(
			err,
			"unable to mkdir: %s", dir,
		))
		return
	}

	storage, err := os.Create(filepath.Join(dir, "data"))
	if err != nil {
		internalError(response, karma.Format(
			err,
			"unable to create data file in %s", dir,
		))
		return
	}

	defer storage.Close()

	_, err = io.Copy(storage, file)
	if err != nil {
		internalError(response, karma.Format(
			err,
			"unable to io copy",
		))
		return
	}

	err = ioutil.WriteFile(
		filepath.Join(dir, "filename"),
		[]byte(header.Filename),
		0644,
	)
	if err != nil {
		internalError(response, karma.Format(
			err,
			"unable to write filename file",
		))
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	err = ioutil.WriteFile(
		filepath.Join(dir, "content_type"),
		[]byte(contentType),
		0644,
	)
	if err != nil {
		internalError(response, karma.Format(
			err,
			"unable to write content_type file",
		))
		return
	}

	rawAutoDelete := request.FormValue("auto_delete")
	if rawAutoDelete == "1" {
		err = ioutil.WriteFile(
			filepath.Join(dir, "auto_delete"),
			[]byte("1"),
			0644,
		)
		if err != nil {
			internalError(response, karma.Format(
				err,
				"unable to write auto_delete file",
			))
			return
		}
	}

	headerDownload := request.FormValue("header_download")
	if headerDownload == "1" {
		err = ioutil.WriteFile(
			filepath.Join(dir, "header_download"),
			[]byte("1"),
			0644,
		)
		if err != nil {
			internalError(response, karma.Format(
				err,
				"unable to write header_download file",
			))
			return
		}
	}

	log.Printf(
		"%s %s %s",
		token,
		header.Filename,
		header.Header.Get("Content-Type"),
	)

	response.Write([]byte(token))
}

func (handler *Handler) download(response http.ResponseWriter, request *http.Request) {
	token := strings.TrimPrefix(request.URL.Path, "/")

	log.Printf("download: %s", token)

	if strings.Contains(token, "/") {
		response.WriteHeader(http.StatusNotFound)
		return
	}

	dir := filepath.Join(handler.dir, token)

	filename, err := ioutil.ReadFile(filepath.Join(dir, "filename"))
	if os.IsNotExist(err) {
		log.Printf("download: not found: %s", token)
		response.WriteHeader(http.StatusNotFound)
		return
	}

	if err != nil {
		internalError(response, err)
		return
	}

	headerDownload := false

	_, err = ioutil.ReadFile(filepath.Join(dir, "header_download"))
	if err == nil {
		headerDownload = true
	} else if !os.IsNotExist(err) {
		internalError(response, err)
		return
	}

	autoDelete := false

	_, err = ioutil.ReadFile(filepath.Join(dir, "auto_delete"))
	if err == nil {
		autoDelete = true
	} else if !os.IsNotExist(err) {
		internalError(response, err)
		return
	}

	if autoDelete {
		if strings.Contains(
			strings.ToLower(request.Header.Get("User-Agent")),
			"bot",
		) {
			response.WriteHeader(http.StatusFound)
			return
		}

		defer func() {
			log.Printf("autoremoving dir: %s", dir)
			err := os.RemoveAll(dir)
			if err != nil {
				log.Println(err)
			}
		}()
	}

	file, err := os.Open(filepath.Join(dir, "data"))
	if err != nil {
		internalError(response, err)
		return
	}

	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		internalError(response, err)
		return
	}

	sniff := make([]byte, 512)
	_, err = file.Read(sniff)
	if err != nil {
		internalError(response, err)
		return
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		internalError(response, err)
		return
	}

	var contentType string
	var contentDisposition string
	if headerDownload {
		contentType = "application/octet-stream"
		contentDisposition = "attachment"
	} else {
		contentType = http.DetectContentType(sniff)
		contentDisposition = "inline"
	}

	response.Header().Set(
		"Content-Type",
		contentType,
	)
	response.Header().Set(
		"Content-Disposition",
		contentDisposition+"; filename="+filepath.Base(string(filename)),
	)
	response.Header().Set(
		"Content-Length",
		fmt.Sprint(stat.Size()),
	)

	_, err = io.Copy(response, file)
	if err != nil {
		internalError(response, err)
		return
	}
}

func internalError(response http.ResponseWriter, err error) {
	log.Println(err)
	response.WriteHeader(http.StatusInternalServerError)
}

func (handler *Handler) getTokenDir() (string, string) {
	for {
		token := randomString(handler.length)
		dir := filepath.Join(handler.dir, token)

		if isFileExists(dir) {
			continue
		}

		return token, dir
	}
}

func isFileExists(path string) bool {
	stat, err := os.Stat(path)
	return !os.IsNotExist(err) && !stat.IsDir()
}

func randomString(length int) string {
	const symbols = "" +
		"qwertyuiopasdfghjklzxcvbnm" +
		"QWERTYUIOPASDFGHJKLZXCVBNM" +
		"1234567890"
	result := ""
	for i := 0; i < length; i++ {
		result += string(symbols[rand.Intn(len(symbols))])
	}

	return result
}
