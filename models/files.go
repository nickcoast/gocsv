package models

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"strings"
)

type File struct {
	File   multipart.File
	Header *multipart.FileHeader
}

func (f File) GetMaxColumnLengths() ([]int, []int, error) {
	csvReader := csv.NewReader(f.File)

	headerRow, err := csvReader.Read()
	if err != nil {
		log.Println("Error reading CSV header:", err)
		return nil, nil, err
	}

	maxLengths := []int{}
	headerLengths := make([]int, len(headerRow))
	// header row lengths
	for i, cell := range headerRow {
		headerLengths[i] = len(cell)
	}

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("Error reading CSV:", err)
			return nil, nil, err
		}

		for i, cell := range row {
			cellLength := len(cell)
			if i >= len(maxLengths) {
				maxLengths = append(maxLengths, cellLength)
			} else if cellLength > maxLengths[i] {
				maxLengths[i] = cellLength
			}
		}
	}

	return maxLengths, headerLengths, nil
}

func (f File) RemoveBOM(file io.Reader) io.Reader {
	const bom = '\uFEFF'
	buffer := new(bytes.Buffer)
	reader := bufio.NewReader(file)
	io.TeeReader(reader, buffer)
	r, _, err := reader.ReadRune()
	if err != nil {
		return file
	}
	if r != bom {
		return io.MultiReader(bytes.NewReader([]byte(string(r))), buffer)
	}
	return buffer
}

func (f File) CalculateFileHash(file io.Reader) (string, error) {
	hash := sha256.New()
	_, err := io.Copy(hash, file)
	if err != nil {
		return "", fmt.Errorf("error calculating file hash: %w", err)
	}

	hashBytes := hash.Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}

// TODO: add removal of empty columns
func (f File) trimAndRemoveBOM(file io.Reader) (io.Reader, error) {
	file = f.RemoveBOM(file)
	return f.RemoveEmptyRows(file)
}

func (f File) RemoveEmptyRows(file io.Reader) (io.Reader, error) {
	reader := csv.NewReader(file)
	var cleanedData bytes.Buffer
	writer := csv.NewWriter(&cleanedData)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV file: %w", err)
		}

		// Remove empty rows and columns
		trimmedRecord := []string{}
		for _, column := range record {
			trimmedColumn := strings.TrimSpace(column)
			if trimmedColumn != "" {
				trimmedRecord = append(trimmedRecord, trimmedColumn)
			}
		}

		if len(trimmedRecord) > 0 {
			err = writer.Write(trimmedRecord)
			if err != nil {
				return nil, fmt.Errorf("error writing cleaned CSV data: %w", err)
			}
		}
	}
	writer.Flush()
	return bytes.NewReader(cleanedData.Bytes()), nil
}
