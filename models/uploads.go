package models

import (
	"context"
	"fmt"
	"time"
)

type Upload struct {
	Id               int64     `json:"id"`
	FileName         string    `json:"source_filename"`
	FileSize         int64     `json:"file_size"`
	ImportFormat     string    `json:"format_name"`
	DatetimeUploaded time.Time `json:"datetime_uploaded"`

}

type UploadModel struct {
	DB *DB
}

//func handleFileUpload(w http.ResponseWriter, r *http.Request, db *models.DB) {
func (m UploadModel) All(ctx context.Context) ([]Upload, error) {	
	query := `SELECT u.id, u.source_filename, u.file_size, u.datetime_uploaded, COALESCE(c.name, '') as format_name
	FROM core_raw_tables u
	LEFT JOIN core_import_formats c ON u.format_id = c.id
	ORDER BY u.datetime_uploaded DESC;`
	rows, err :=m.DB.QueryWithContext(ctx, query)	
	if err != nil {		
		return nil, fmt.Errorf("Failed to fetch file information from the database")
	}
	defer rows.Close()

	fileInfos := []Upload{}

	for rows.Next() {
		var fileInfo Upload
		err := rows.Scan(&fileInfo.Id, &fileInfo.FileName, &fileInfo.FileSize, &fileInfo.DatetimeUploaded, &fileInfo.ImportFormat)
		if err != nil {
			return nil, fmt.Errorf("Failed to read file information from the database")
		}
		fileInfos = append(fileInfos, fileInfo)
	}
	return fileInfos, nil
}


func (m UploadModel) Delete(ctx context.Context, id int) (error) {
	tx, err := m.DB.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("Failed to start transaction")
	}
	defer m.DB.Close()
	_, err = tx.ExecContext(ctx, "DELETE FROM core_raw_tables WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("Failed to delete file")
	}
	return nil	
}