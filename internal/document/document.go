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
	TimeoutSeconds   string   `yaml:"timeoutSeconds" json:"timeoutSeconds"`
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
	TimeoutSeconds   string               `yaml:"timeoutSeconds,omitempty" json:"timeoutSeconds,omitempty"`
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
func (d *Document) IsDeployed() bool {
	_, err := d.clients.ssm.GetDocument(&ssm.GetDocumentInput{
		Name: &d.Name,
	})
	return err == nil
}

// GetShellContent return document content for shell document
func (d *Document) GetShellContent() (string, error) {
	// Load script content
	fileContent, err := ioutil.ReadFile(d.File)
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
		Description:   d.Description,
		Parameters:    d.Parameters,
		MainSteps: []MainStep{
			{
				Action: "aws:runShellScript",
				Name:   "RunShellScript",
				Inputs: ShellInput{
					WorkingDirectory: d.WorkingDirectory,
					RunCommand:       strings.Split(string(fileContent), EOL),
					TimeoutSeconds:   d.TimeoutSeconds,
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
func (d *Document) GetContent() (*string, *string, error) {

	// Check for shell command
	if d.Format == "SHELL" {
		format := "JSON"
		content, err := d.GetShellContent()
		if err != nil {
			return &format, nil, err
		}
		return &format, &content, nil
	}

	// Check for provided content
	if len(d.Content.SchemaVersion) > 0 {
		format := "JSON"
		marshal, err := jsoniter.Marshal(d.Content)
		if err != nil {
			return &format, nil, err
		}

		content := string(marshal)
		return &format, &content, nil
	}

	// Check for provided file
	if len(d.File) > 0 {
		var format string

		// Check extension
		fileExtension := filepath.Ext(d.File)
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
			content, err := d.GetShellContent()
			if err != nil {
				return &format, nil, err
			}
			return &format, &content, nil
		default:
			return nil, nil, fmt.Errorf("Provided file %s has an unsupported extension", d.File)
		}

		// Load file content
		fileContent, err := ioutil.ReadFile(d.File)
		if err != nil {
			return nil, nil, err
		}

		// Return file content
		content := string(fileContent)
		return &format, &content, nil
	}

	return nil, nil, errors.New("Cannot generate SSM document")
}

// GetExplodedAccountIDs return account ids with comma separated values exploded
func (d *Document) GetExplodedAccountIDs() []string {
	accountIDs := []string{}

	// Loop over configured account ids
	for _, accountID := range d.AccountIDs {
		// Check if contains a comman
		if strings.Contains(accountID, ",") {
			// Explode string, trim parts and add to ids slice
			accountIDsSplit := strings.Split(accountID, ",")
			for _, accountIDSplit := range accountIDsSplit {
				trimStr := strings.Trim(accountIDSplit, " ")
				if len(trimStr) > 0 {
					accountIDs = append(accountIDs, trimStr)
				}
			}
		} else {
			// Add single account
			trimStr := strings.Trim(accountID, " ")
			if len(trimStr) > 0 {
				accountIDs = append(accountIDs, trimStr)
			}
		}
	}

	// Remove duplication
	keys := make(map[string]bool)
	uniqueAccountIDs := []string{}
	for _, entry := range accountIDs {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			uniqueAccountIDs = append(uniqueAccountIDs, entry)
		}
	}

	// Return unique account ids slice
	return uniqueAccountIDs
}

// Deploy document
func (d *Document) Deploy() error {

	// Get content
	format, content, err := d.GetContent()
	if err != nil {
		return err
	}

	// Check if Document is already deployed
	if d.IsDeployed() == false {
		input := &ssm.CreateDocumentInput{
			Name:           &d.Name,
			DocumentFormat: format,
			DocumentType:   &d.Type,
			Content:        content,
		}

		// Parse tag
		for key, value := range d.Tags {
			input.Tags = append(input.Tags, &ssm.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}

		// Create document
		_, err := d.clients.ssm.CreateDocument(input)
		if err != nil {
			return err
		}
	} else {
		input := &ssm.UpdateDocumentInput{
			Name:            &d.Name,
			DocumentFormat:  format,
			Content:         content,
			DocumentVersion: aws.String("$LATEST"),
		}

		// Update document
		res, err := d.clients.ssm.UpdateDocument(input)
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				if awsErr.Code() != ssm.ErrCodeDuplicateDocumentContent {
					return err
				}
			}
		} else {
			// Update latest document version
			_, err = d.clients.ssm.UpdateDocumentDefaultVersion(&ssm.UpdateDocumentDefaultVersionInput{
				Name:            &d.Name,
				DocumentVersion: res.DocumentDescription.DocumentVersion,
			})
			if err != nil {
				return err
			}
		}
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
	permRes, err := d.clients.ssm.DescribeDocumentPermission(&ssm.DescribeDocumentPermissionInput{
		Name:           &d.Name,
		PermissionType: aws.String("Share"),
	})
	if err != nil {
		return err
	}

	// Skip permissions if not set
	accountIDs := d.GetExplodedAccountIDs()
	if len(accountIDs) == 0 && len(permRes.AccountIds) == 0 {
		return nil
	}

	// Check accounts ids to add
	accountsToAdd := []string{}
	for _, accountID := range accountIDs {
		var foundAccount string
		for _, currentAccountID := range permRes.AccountIds {
			if accountID == *currentAccountID {
				foundAccount = accountID
				break
			}
		}

		if len(foundAccount) == 0 {
			accountsToAdd = append(accountsToAdd, accountID)
		}
	}

	// Check accounts ids to remove
	accountsToRemove := []string{}
	for _, currentAccountID := range permRes.AccountIds {
		var foundAccount string
		for _, accountID := range accountIDs {
			if accountID == *currentAccountID {
				foundAccount = accountID
				break
			}
		}

		if len(foundAccount) == 0 {
			accountsToRemove = append(accountsToRemove, *currentAccountID)
		}
	}

	// Update permissions if needed
	if len(accountsToAdd) > 0 || len(accountsToRemove) > 0 {
		// Build update input
		updateInput := &ssm.ModifyDocumentPermissionInput{
			Name:           &d.Name,
			PermissionType: aws.String("Share"),
		}
		if len(accountsToAdd) > 0 {
			updateInput.AccountIdsToAdd = aws.StringSlice(accountsToAdd)
		}
		if len(accountsToRemove) > 0 {
			updateInput.AccountIdsToRemove = aws.StringSlice(accountsToRemove)
		}

		// Execute update
		_, err = d.clients.ssm.ModifyDocumentPermission(updateInput)
		return err
	}

	return nil
}

// UpdateTags update canary tags
func (d *Document) UpdateTags() error {
	// Get current tags
	resTags, err := d.clients.ssm.ListTagsForResource(&ssm.ListTagsForResourceInput{
		ResourceId:   &d.Name,
		ResourceType: aws.String("Document"),
	})
	if err != nil {
		return err
	}

	// Parse document tags
	tags := []*ssm.Tag{}
	for key, value := range d.Tags {
		tags = append(tags, &ssm.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}

	// Skip if not tags are set
	if len(tags) == 0 && len(resTags.TagList) == 0 {
		return nil
	}

	// Check tags to add
	tagsToAdd := []*ssm.Tag{}
	for _, tag := range tags {
		var foundTag *ssm.Tag
		for _, currentTag := range resTags.TagList {
			if *tag.Key == *currentTag.Key && *tag.Value == *currentTag.Value {
				foundTag = tag
				break
			}
		}

		if foundTag == nil {
			tagsToAdd = append(tagsToAdd, tag)
		}
	}

	// Add missing tags
	if len(tagsToAdd) > 0 {
		_, err = d.clients.ssm.AddTagsToResource(&ssm.AddTagsToResourceInput{
			ResourceId:   &d.Name,
			ResourceType: aws.String("Document"),
			Tags:         tagsToAdd,
		})
		if err != nil {
			return err
		}
	}

	// Check accounts ids to remove
	tagsKeysToRemove := []*string{}
	for _, currentTag := range resTags.TagList {
		var foundTagKey *string
		for _, tag := range tags {
			if *tag.Key == *currentTag.Key {
				foundTagKey = tag.Key
				break
			}
		}

		if foundTagKey == nil {
			tagsKeysToRemove = append(tagsKeysToRemove, currentTag.Key)
		}
	}

	// Remove unused tags
	if len(tagsKeysToRemove) > 0 {
		_, err = d.clients.ssm.RemoveTagsFromResource(&ssm.RemoveTagsFromResourceInput{
			ResourceId:   &d.Name,
			ResourceType: aws.String("Document"),
			TagKeys:      tagsKeysToRemove,
		})
	}

	return err
}

// Remove document
func (d *Document) Remove() error {
	// Retrieve current document permissions
	permRes, err := d.clients.ssm.DescribeDocumentPermission(&ssm.DescribeDocumentPermissionInput{
		Name:           &d.Name,
		PermissionType: aws.String("Share"),
	})
	if err != nil {
		return err
	}

	// Remove all permissions
	if len(permRes.AccountIds) > 0 {
		d.clients.ssm.ModifyDocumentPermission(&ssm.ModifyDocumentPermissionInput{
			Name:               &d.Name,
			AccountIdsToRemove: permRes.AccountIds,
		})
		return nil
	}

	// Delete document
	_, err = d.clients.ssm.DeleteDocument(&ssm.DeleteDocumentInput{
		Name: &d.Name,
	})
	if err != nil {
		return err
	}

	return nil
}
