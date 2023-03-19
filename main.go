package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"database/sql"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
)

func main() {

	db, err := connectToDatabase()
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer db.Close()

	r := mux.NewRouter()

	r.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
        handleFileUpload(w, r, db)
    }).Methods("POST", "OPTIONS")
    r.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
        fetchUploadedFiles(w, r, db)
    }).Methods("GET", "OPTIONS")
    r.HandleFunc("/files/{id}", func(w http.ResponseWriter, r *http.Request) {
        deleteFile(w, r, db)
    }).Methods("DELETE", "OPTIONS")

	/* r.HandleFunc("/upload", handleFileUpload).Methods("POST")
	r.HandleFunc("/files", fetchUploadedFiles).Methods("GET")
	r.HandleFunc("/files/{id}", func(w http.ResponseWriter, r *http.Request) {
		deleteFile(w, r, db)
	  }).Methods("DELETE")
 */
	// Add CORS middleware
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"}, // Change this to the appropriate origin in production
		AllowedMethods:   []string{"POST", "GET", "PUT", "DELETE"},
		AllowCredentials: true,
	})

	handler := c.Handler(r)

	port := ":8080"
	fmt.Println("Listening on port", port)
	log.Fatal(http.ListenAndServe(port, handler))
}

func handleFileUpload(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	err := r.ParseMultipartForm(32 << 20) // 32 MB
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	 // Check the MIME type of the uploaded file
	 buffer := make([]byte, 512)
	 _, err = file.Read(buffer)
	 if err != nil {
	   http.Error(w, "Failed to read file", http.StatusBadRequest)
	   return
	 }
	 contentType := http.DetectContentType(buffer)
	 if contentType != "text/csv" && !strings.HasPrefix(contentType, "image/") {
	   http.Error(w, "Invalid file type. Only CSV and image files are allowed", http.StatusBadRequest)
	   return
	 }
	 _, err = file.Seek(0, io.SeekStart) // Reset the file read position
	 if err != nil {
	   http.Error(w, "Failed to read file", http.StatusBadRequest)
	   return
	 }


	uploadPath := filepath.Join("uploads", handler.Filename)
	err = os.MkdirAll(filepath.Dir(uploadPath), os.ModePerm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tempFile, err := os.CreateTemp("uploads", handler.Filename+"_tmp_*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = tempFile.Write(fileBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = os.Rename(tempFile.Name(), uploadPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	query := "INSERT INTO uploaded_files (file_name, file_size, datetime_uploaded) VALUES ($1, $2, $3)"
	_, err = db.Exec(query, handler.Filename, handler.Size, time.Now())
	fmt.Println(query, handler.Filename, handler.Size, handler.Header, time.Now())
	if err != nil {
		http.Error(w, "Failed to save file information to the database", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "File uploaded successfully: %s", handler.Filename)
}

func connectToDatabase() (*sql.DB, error) {
	connStr := "user=ogrego password=vagrant dbname=ogrego host=localhost sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Add a new function to fetch file information from the database
func fetchUploadedFiles(w http.ResponseWriter, r *http.Request, db *sql.DB) {	
	query := "SELECT file_name, file_size, datetime_uploaded FROM uploaded_files ORDER BY datetime_uploaded DESC"
	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, "Failed to fetch file information from the database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type FileInfo struct {
		FileName         string    `json:"file_name"`
		FileSize         int64     `json:"file_size"`
		DatetimeUploaded time.Time `json:"datetime_uploaded"`
	}

	fileInfos := []FileInfo{}

	for rows.Next() {
		var fileInfo FileInfo
		err := rows.Scan(&fileInfo.FileName, &fileInfo.FileSize, &fileInfo.DatetimeUploaded)
		if err != nil {
			http.Error(w, "Failed to read file information from the database", http.StatusInternalServerError)
			return
		}
		fileInfos = append(fileInfos, fileInfo)
	}

	json.NewEncoder(w).Encode(fileInfos)
}

func deleteFile(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	vars := mux.Vars(r)
	id := vars["id"]
  
	_, err := db.Exec("DELETE FROM uploaded_files WHERE id = $1", id)
	if err != nil {
	  http.Error(w, "Failed to delete file", http.StatusInternalServerError)
	  return
	}
  
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("File deleted successfully"))
  }