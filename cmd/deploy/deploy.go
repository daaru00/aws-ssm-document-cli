package deploy

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/daaru00/aws-ssm-document-cli/internal/aws"
	"github.com/daaru00/aws-ssm-document-cli/internal/config"
	"github.com/daaru00/aws-ssm-document-cli/internal/document"
	"github.com/urfave/cli/v2"
)

// NewCommand - Return deploy commands
func NewCommand(globalFlags []cli.Flag) *cli.Command {
	return &cli.Command{
		Name:    "deploy",
		Aliases: []string{"up"},
		Usage:   "Deploy SSM Documents",
		Flags: append(globalFlags, []cli.Flag{
			&cli.StringFlag{
				Name:    "artifact-bucket",
				Usage:   "Then artifact bucket name",
				EnvVars: []string{"SSM_DOCUMENT_ARTIFACT_BUCKET", "SSM_DOCUMENT_ARTIFACT_BUCKET_NAME"},
			},
			&cli.StringFlag{
				Name:    "sources-bucket",
				Usage:   "Then source code bucket name",
				EnvVars: []string{"SSM_DOCUMENT_SOURCES_BUCKET", "SSM_DOCUMENT_SOURCES_BUCKET_NAME"},
			},
			&cli.BoolFlag{
				Name:    "yes",
				Aliases: []string{"y"},
				Usage:   "Answer yes for all confirmations",
			},
			&cli.BoolFlag{
				Name:    "build",
				Aliases: []string{"b"},
				Usage:   "Build document before deploy",
			},
			&cli.BoolFlag{
				Name:    "upload",
				Aliases: []string{"u"},
				Usage:   "Upload code to source bucket",
			},
			&cli.BoolFlag{
				Name:    "start",
				Aliases: []string{"s"},
				Usage:   "Start document after deploy",
			},
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "Select all documents",
			},
			&cli.IntFlag{
				Name:  "parallels",
				Usage: "Set max deploy executed in parallel",
				Value: 5,
			},
		}...),
		Action:    Action,
		ArgsUsage: "[path...]",
	}
}

// Action contain the command flow
func Action(c *cli.Context) error {
	// Create AWS session
	ses := aws.NewAwsSession(c)

	// Get caller infos
	accountID := aws.GetCallerAccountID(ses)
	region := aws.GetCallerRegion(ses)
	if accountID == nil {
		return errors.New("No valid AWS credentials found")
	}

	// Get document
	documents, err := config.LoadDocuments(c, ses)
	if err != nil {
		return err
	}

	// Ask document selection
	documents, err = config.AskMultipleDocumentsSelection(c, *documents)
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

				err := deploySingleDocument(ses, region, accountID, document)

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
		return fmt.Errorf("%d of %d document fail deploy", inError, len(*documents))
	}

	return nil
}

func deploySingleDocument(ses *session.Session, region *string, accountID *string, document *document.Document) error {
	var err error

	isAlreadyDeployed := document.IsDeployed()

	// Deploy document
	if !isAlreadyDeployed {
		fmt.Println(fmt.Sprintf("[%s] Creating..", document.Name))
	} else {
		fmt.Println(fmt.Sprintf("[%s] Updating..", document.Name))
	}
	err = document.Deploy()
	if err != nil {
		return err
	}

	// Update tags
	if isAlreadyDeployed {
		fmt.Println(fmt.Sprintf("[%s] Updating tags..", document.Name))
		err = document.UpdateTags()
		if err != nil {
			return err
		}
	}

	fmt.Println(fmt.Sprintf("[%s] Deploy completed!", document.Name))
	return nil
}
