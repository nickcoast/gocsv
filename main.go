package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func uploadFile(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Uploading File")

	r.ParseMultipartForm(10 << 20)

	file, handler, err := r.FormFile("myFile")
	if err != nil {
		fmt.Println("Error retrieving the file")
		fmt.Println(err)
		return
	}
	defer file.Close()
	fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	fmt.Printf("File Size: %+v\n", handler.Size)
	fmt.Printf("MIME Header:  %+v\n", handler.Header)

	tempFile, err := os.CreateTemp("csv","upload-*.csv")
	if err != nil {
		fmt.Println(err)
	}
	defer tempFile.Close()

	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println(err)
	}

	tempFile.Write(fileBytes)

	fmt.Fprint(w,"Successfully uploaded file\n")
}

func setupRoutes() {
	http.HandleFunc("/uplaod", uploadFile)
	http.ListenAndServe(":8080", nil)
}

func main() {
	fmt.Println("Hell Ow Orld!")
	setupRoutes()
}
