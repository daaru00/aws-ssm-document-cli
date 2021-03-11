package document

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	jsoniter "github.com/json-iterator/go"
)

type clients struct {
	ssm *ssm.SSM
}

// ShellInput content for shell document
type ShellInput struct {
	WorkingDirectory string   `yaml:"workingDirectory" json:"workingDirectory"`
	RunCommand       []string `yaml:"runCommand" json:"runCommand"`
}

// MainStep content for document
type MainStep struct {
	Action string      `yaml:"action" json:"action"`
	Name   string      `yaml:"name" json:"name"`
	Inputs interface{} `yaml:"inputs,omitempty" json:"inputs,omitempty"`
}

// Content document
type Content struct {
	SchemaVersion string               `yaml:"schemaVersion" json:"schemaVersion"`
	Description   string               `yaml:"description" json:"description"`
	Parameters    map[string]Parameter `yaml:"parameters" json:"parameters"`
	MainSteps     []MainStep           `yaml:"mainSteps" json:"mainSteps"`
}

// Parameter configuration
type Parameter struct {
	Type        string `yaml:"type" json:"type"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Default     string `yaml:"default,omitempty" json:"default,omitempty"`
}

// Document structure
type Document struct {
	clients *clients
	region  *string

	Name             string               `yaml:"name" json:"name"`
	Description      string               `yaml:"description" json:"description"`
	Type             string               `yaml:"type" json:"type"`
	AccountIDs       []string             `yaml:"accountIds" json:"accountIds"`
	Tags             map[string]string    `yaml:"tags" json:"tags"`
	Content          Content              `yaml:"content,omitempty" json:"content,omitempty"`
	Parameters       map[string]Parameter `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	WorkingDirectory string               `yaml:"workingDirectory" json:"workingDirectory"`
	Format           string               `yaml:"format" json:"format"`
	File             string               `yaml:"file" json:"file"`
}

// New creates a new Document
func New(ses *session.Session, name string) *Document {
	clients := &clients{
		ssm: ssm.New(ses),
	}

	return &Document{
		clients: clients,
		region:  ses.Config.Region,

		Name: name,
		Type: "Command",
	}
}

// IsDeployed check if document name is present in current AWS account
func (c *Document) IsDeployed() bool {
	_, err := c.clients.ssm.GetDocument(&ssm.GetDocumentInput{
		Name: &c.Name,
	})
	return err == nil
}

// Deploy document
func (c *Document) Deploy() error {

	// Get content
	format, content, err := c.GetContent()
	if err != nil {
		return err
	}

	// Check if Document is already deployed
	if c.IsDeployed() == false {
		input := &ssm.CreateDocumentInput{
			Name:           &c.Name,
			DocumentFormat: format,
			DocumentType:   &c.Type,
			Content:        content,
		}

		// Parse tag
		for key, value := range c.Tags {
			input.Tags = append(input.Tags, &ssm.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}

		// Create document
		_, err = c.clients.ssm.CreateDocument(input)
	} else {
		input := &ssm.UpdateDocumentInput{
			Name:            &c.Name,
			DocumentFormat:  format,
			Content:         content,
			DocumentVersion: aws.String("$LATEST"),
		}

		// Update document
		_, err = c.clients.ssm.UpdateDocument(input)
	}

	// Check for deploy error
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() != ssm.ErrCodeDuplicateDocumentContent {
				return err
			}
		}
	}

	// Retrieve current document permissions
	permRes, err := c.clients.ssm.DescribeDocumentPermission(&ssm.DescribeDocumentPermissionInput{
		Name:           &c.Name,
		PermissionType: aws.String("Share"),
	})
	if err != nil {
		return err
	}

	// Skip permissions
	if len(c.AccountIDs) == 0 && len(permRes.AccountIds) == 0 {
		return nil
	}

	// Add all permissions
	if len(c.AccountIDs) > 0 && len(permRes.AccountIds) == 0 {
		_, err := c.clients.ssm.ModifyDocumentPermission(&ssm.ModifyDocumentPermissionInput{
			Name:            &c.Name,
			PermissionType:  aws.String("Share"),
			AccountIdsToAdd: aws.StringSlice(c.AccountIDs),
		})
		return err
	}

	// Remove all permissions
	if len(c.AccountIDs) == 0 && len(permRes.AccountIds) > 0 {
		_, err := c.clients.ssm.ModifyDocumentPermission(&ssm.ModifyDocumentPermissionInput{
			Name:               &c.Name,
			PermissionType:     aws.String("Share"),
			AccountIdsToRemove: permRes.AccountIds,
		})
		return err
	}

	// Check accounts ids to add
	accountToAdd := []string{}
	for _, accountID := range c.AccountIDs {
		var foundAccount string
		for _, currentAccountID := range permRes.AccountIds {
			if accountID == *currentAccountID {
				foundAccount = accountID
				break
			}
		}

		if len(foundAccount) == 0 {
			accountToAdd = append(accountToAdd, accountID)
		}
	}

	// Check accounts ids to remove
	accountToRemove := []string{}
	for _, currentAccountID := range permRes.AccountIds {
		var foundAccount string
		for _, accountID := range c.AccountIDs {
			if accountID == *currentAccountID {
				foundAccount = accountID
				break
			}
		}

		if len(foundAccount) == 0 {
			accountToRemove = append(accountToRemove, *currentAccountID)
		}
	}

	// Update permissions if needed
	if len(accountToAdd) > 0 || len(accountToRemove) > 0 {
		_, err = c.clients.ssm.ModifyDocumentPermission(&ssm.ModifyDocumentPermissionInput{
			Name:               &c.Name,
			PermissionType:     aws.String("Share"),
			AccountIdsToAdd:    aws.StringSlice(accountToAdd),
			AccountIdsToRemove: aws.StringSlice(accountToRemove),
		})
		return err
	}

	return nil
}

// Remove document
func (c *Document) Remove() error {
	// Retrieve current document permissions
	permRes, err := c.clients.ssm.DescribeDocumentPermission(&ssm.DescribeDocumentPermissionInput{
		Name:           &c.Name,
		PermissionType: aws.String("Share"),
	})
	if err != nil {
		return err
	}

	// Remove all permissions
	if len(permRes.AccountIds) > 0 {
		c.clients.ssm.ModifyDocumentPermission(&ssm.ModifyDocumentPermissionInput{
			Name:               &c.Name,
			AccountIdsToRemove: permRes.AccountIds,
		})
		return nil
	}

	// Delete document
	_, err = c.clients.ssm.DeleteDocument(&ssm.DeleteDocumentInput{
		Name: &c.Name,
	})
	if err != nil {
		return err
	}

	return nil
}

// GetShellContent return document content for shell document
func (c *Document) GetShellContent() (string, error) {
	// Load script content
	fileContent, err := ioutil.ReadFile(c.File)
	if err != nil {
		return "", err
	}

	// Line char
	var EOL string
	switch runtime.GOOS {
	case "windows":
		EOL = "\r\n"
		break
	default:
		EOL = "\n"
	}

	// Build content
	content := Content{
		SchemaVersion: "2.2",
		Description:   c.Description,
		Parameters:    c.Parameters,
		MainSteps: []MainStep{
			{
				Action: "aws:runShellScript",
				Name:   "RunShellScript",
				Inputs: ShellInput{
					WorkingDirectory: c.WorkingDirectory,
					RunCommand:       strings.Split(string(fileContent), EOL),
				},
			},
		},
	}

	// Convert into JSON string
	marshal, err := jsoniter.Marshal(content)
	if err != nil {
		return "", err
	}

	return string(marshal), nil
}

// GetContent return the SSM content
func (c *Document) GetContent() (*string, *string, error) {

	// Check for shell command
	if c.Format == "SHELL" {
		format := "JSON"
		content, err := c.GetShellContent()
		if err != nil {
			return &format, nil, err
		}
		return &format, &content, nil
	}

	// Check for provided content
	if len(c.Content.SchemaVersion) > 0 {
		format := "JSON"
		marshal, err := jsoniter.Marshal(c.Content)
		if err != nil {
			return &format, nil, err
		}

		content := string(marshal)
		return &format, &content, nil
	}

	// Check for provided file
	if len(c.File) > 0 {
		fmt.Println("3")
		var format string

		// Check extension
		fileExtension := filepath.Ext(c.File)
		switch strings.ToUpper(fileExtension) {
		case ".TXT":
			format = "TEXT"
			break
		case ".YML":
		case ".YAML":
			format = "YAML"
			break
		case ".JSON":
			format = "JSON"
		case ".SH":
			format := "JSON"
			content, err := c.GetShellContent()
			if err != nil {
				return &format, nil, err
			}
			return &format, &content, nil
		default:
			return nil, nil, fmt.Errorf("Provided file %s has an unsupported extension", c.File)
		}

		// Load file content
		fileContent, err := ioutil.ReadFile(c.File)
		if err != nil {
			return nil, nil, err
		}

		// Return file content
		content := string(fileContent)
		return &format, &content, nil
	}

	return nil, nil, errors.New("Cannot generate SSM document")
}
