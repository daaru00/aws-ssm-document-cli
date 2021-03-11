package config

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/daaru00/aws-ssm-document-cli/internal/document"
	"github.com/urfave/cli/v2"
)

// AskMultipleDocumentsSelection ask user to select multiple documents
func AskMultipleDocumentsSelection(c *cli.Context, documents []*document.Document) (*[]*document.Document, error) {
	selectedDocuments := []*document.Document{}

	// Check if single document
	if len(documents) == 1 {
		return &documents, nil
	}

	// Check if all flag is present
	if c.Bool("all") {
		return &documents, nil
	}

	// Build table
	header := fmt.Sprintf("%-25s", "Name")
	var options []string
	for _, document := range documents {
		options = append(options, fmt.Sprintf("%-20s", document.Name))
	}

	// Ask selection
	documentsSelectedIndexes := []int{}
	prompt := &survey.MultiSelect{
		Message:  "Select documents: \n\n  " + header + "\n",
		Options:  options,
		PageSize: 15,
	}
	survey.AskOne(prompt, &documentsSelectedIndexes)
	fmt.Println("")

	// Check response
	if len(documentsSelectedIndexes) == 0 {
		return &selectedDocuments, errors.New("No documents selected")
	}

	// Load selected documents
	for _, index := range documentsSelectedIndexes {
		selectedDocuments = append(selectedDocuments, documents[index])
	}

	return &selectedDocuments, nil
}
