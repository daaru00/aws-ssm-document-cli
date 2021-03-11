package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/daaru00/aws-ssm-document-cli/internal/document"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"
)

// LoadDotEnv will load environment variable from .env file
func LoadDotEnv() error {
	env := os.Getenv("SSM_DOCUMENT_ENV")
	envFile := ".env"

	// Build env file name
	if len(env) != 0 {
		envFile += "." + env
	}

	// Check if file exist
	_, err := os.Stat(envFile)
	if os.IsNotExist(err) {
		return nil
	}

	// Load environment variables
	return godotenv.Load(envFile)
}

// LoadDocuments load document using user input
func LoadDocuments(c *cli.Context, ses *session.Session) (*[]*document.Document, error) {
	documents := []*document.Document{}

	// Search config in sources
	fileName := c.String("config-file")
	parser := c.String("config-parser")

	// Check tests source path argument
	searchPaths := []string{"."}
	if c.Args().Len() > 0 {
		searchPaths = c.Args().Slice()
	} else {
		envVar := os.Getenv("SSM_DOCUMENT_PATH")
		if len(envVar) > 0 {
			searchPaths = []string{envVar}
		}
	}

	// Iterate over search paths provided
	for _, searchPath := range searchPaths {

		// Check provided path type
		info, err := os.Stat(searchPath)
		if err != nil {
			return &documents, err
		}

		// Check if path is a directory or file
		fileMode := info.Mode()
		if fileMode.IsDir() {
			// Found document in directory
			documentsFound, err := LoadDocumentsFromDir(ses, &searchPath, &fileName, &parser)
			if err != nil {
				return nil, err
			}

			// Append documents
			documents = append(documents, documentsFound...)
		} else if fileMode.IsRegular() {
			// Load document from file
			documentFound, err := LoadDocumentFromFile(ses, &searchPath, &parser)
			if err != nil {
				return nil, err
			}

			// Append documents
			documents = append(documents, documentFound)
		} else {
			return &documents, fmt.Errorf("Path %s has a unsupported type", searchPath)
		}
	}

	return &documents, nil
}

// LoadDocumentFromFile load document from file
func LoadDocumentFromFile(ses *session.Session, filePath *string, parser *string) (*document.Document, error) {
	// If file match read content
	fileContent, err := ioutil.ReadFile(*filePath)
	if err != nil {
		return nil, err
	}

	// Interpolate file content
	fileContentInterpolated := InterpolateContent(&fileContent)

	// Parse file content into config object
	fileName := filepath.Base(*filePath)
	extension := filepath.Ext(fileName)
	documentName := fileName[0 : len(fileName)-len(extension)]
	document := document.New(ses, documentName)
	err = ParseContent(fileContentInterpolated, parser, document)
	if err != nil {
		return nil, err
	}

	// If file path is provided convert to absolute
	if len(document.File) > 0 {
		document.File = filepath.Join(filepath.Dir(*filePath), document.File)
	}

	return document, nil
}

// LoadDocumentsFromDir search config files and load documents
func LoadDocumentsFromDir(ses *session.Session, searchPath *string, fileNameToMatch *string, parser *string) ([]*document.Document, error) {
	start := time.Now()
	filesCount := 0
	documents := []*document.Document{}

	// Walk for each files in source path
	err := filepath.Walk(*searchPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}
		filesCount++

		// Check if file match name
		fileName := filepath.Base(filePath)
		match, _ := filepath.Match(*fileNameToMatch, fileName)
		if !match {
			return nil
		}

		// Parse document from file
		document, err := LoadDocumentFromFile(ses, &filePath, parser)
		if err != nil {
			return err
		}
		// Add document to slice
		documents = append(documents, document)
		return nil
	})

	// Check for errors
	if err != nil {
		return documents, err
	}

	// Check documents length
	if len(documents) == 0 {
		round, _ := time.ParseDuration("5ms")
		elapsed := time.Since(start).Round(round)
		return documents, fmt.Errorf("No documents found in path %s (%d files scanned in %s)", *searchPath, filesCount, elapsed)
	}

	// Return documents
	return documents, err
}
