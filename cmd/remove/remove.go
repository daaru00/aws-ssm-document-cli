package remove

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/daaru00/aws-ssm-document-cli/internal/aws"
	"github.com/daaru00/aws-ssm-document-cli/internal/config"
	"github.com/daaru00/aws-ssm-document-cli/internal/document"
	"github.com/urfave/cli/v2"
)

// NewCommand - Return remove commands
func NewCommand(globalFlags []cli.Flag) *cli.Command {
	return &cli.Command{
		Name:    "remove",
		Aliases: []string{"delete", "down"},
		Usage:   "Remove SSM Documents",
		Flags: append(globalFlags, []cli.Flag{
			&cli.StringFlag{
				Name:    "artifact-bucket",
				Usage:   "The Artifact bucket name",
				EnvVars: []string{"SSM_DOCUMENT_ARTIFACT_BUCKET", "SSM_DOCUMENT_ARTIFACT_BUCKET_NAME"},
			},
			&cli.BoolFlag{
				Name:  "delete-artifact-bucket",
				Usage: "Remove also artifact bucket",
			},
			&cli.StringFlag{
				Name:    "sources-bucket",
				Usage:   "Then source code bucket name",
				EnvVars: []string{"SSM_DOCUMENT_SOURCES_BUCKET", "SSM_DOCUMENT_SOURCES_BUCKET_NAME"},
			},
			&cli.BoolFlag{
				Name:  "delete-sources-bucket",
				Usage: "Remove also source bucket",
			},
			&cli.BoolFlag{
				Name:    "yes",
				Aliases: []string{"y"},
				Usage:   "Answer yes for all confirmations",
			},
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "Select all documents",
			},
			&cli.IntFlag{
				Name:  "parallels",
				Usage: "Set max remove executed in parallel",
				Value: 5,
			},
		}...),
		Action:    Action,
		ArgsUsage: "[path...]",
	}
}

// Action contain the command flow
func Action(c *cli.Context) error {
	var err error

	// Create AWS session
	ses := aws.NewAwsSession(c)

	// Get caller infos
	accountID := aws.GetCallerAccountID(ses)
	region := aws.GetCallerRegion(ses)
	if accountID == nil {
		return errors.New("No valid AWS credentials found")
	}

	// Get documents
	documents, err := config.LoadDocuments(c, ses)
	if err != nil {
		return err
	}

	// Ask documents selection
	documents, err = config.AskMultipleDocumentsSelection(c, *documents)
	if err != nil {
		return err
	}

	// Ask confirmation
	err = askConfirmation(c, fmt.Sprintf("Are you sure you want to remove %d documents?", len(*documents)))
	if err != nil {
		return err
	}

	// Setup errors channel
	errs := make(chan error, len(*documents))

	// Setup max parallels
	allDocuments := *documents
	parallels := c.Int("parallels")

	// Start parallel deploy
	for start := 0; start < len(allDocuments); start += parallels {
		// Update chunk start and end
		end := start + parallels
		if end > len(allDocuments) {
			end = len(allDocuments)
		}

		// Split array chunk
		chunk := allDocuments[start:end]

		// Setup wait group for async jobs
		var waitGroup sync.WaitGroup

		// Loop over found document
		for _, cy := range chunk {

			// Execute parallel deploy
			waitGroup.Add(1)
			go func(document *document.Document) {
				defer waitGroup.Done()

				err := removeSingleDocument(ses, document, region)

				errs <- err
			}(cy)
		}

		// Wait until all remove ends
		waitGroup.Wait()

		// Do dummy wait
		time.Sleep(2 * time.Second)
	}

	// Close errors channel
	close(errs)

	// Check errors
	var inError int
	for i := 0; i < len(*documents); i++ {
		err := <-errs
		if err != nil {
			inError++
			fmt.Println(err)
		}
	}
	if inError > 0 {
		return fmt.Errorf("%d of %d documents fail remove", inError, len(*documents))
	}

	return nil
}

func removeSingleDocument(ses *session.Session, document *document.Document, region *string) error {
	var err error

	if document.IsDeployed() {
		// Remove document
		fmt.Println(fmt.Sprintf("[%s] Removing..", document.Name))
		err = document.Remove()
		if err != nil {
			return err
		}
	}

	fmt.Println(fmt.Sprintf("[%s] Remove completed!", document.Name))
	return nil
}

func askConfirmation(c *cli.Context, message string) error {
	// Check yes flag
	if c.Bool("yes") {
		return nil
	}

	// Ask confirmation
	confirm := false
	prompt := &survey.Confirm{
		Message: message,
	}
	survey.AskOne(prompt, &confirm)

	// Check respose
	if confirm == false {
		return errors.New("Not confirmed documents remove, skip operation")
	}

	return nil
}
