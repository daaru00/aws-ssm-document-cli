package aws

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/urfave/cli/v2"
)

// NewAwsSession return a new AWS client session
func NewAwsSession(c *cli.Context) *session.Session {
	profile := c.String("profile")
	region := c.String("region")

	// Create AWS config object
	awsConfig := aws.Config{}
	if len(region) != 0 {
		awsConfig.Region = aws.String(region)
	}

	// Check for debug mode
	debugMode := os.Getenv("AWS_DEBUG")
	if debugMode != "" {
		awsConfig.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	// Return a new session
	return session.Must(session.NewSessionWithOptions(session.Options{
		Profile:           profile,
		SharedConfigState: session.SharedConfigEnable,
		Config:            awsConfig,
	}))
}

// GetCallerAccountID return the account number
func GetCallerAccountID(ses *session.Session) *string {
	stsClient := sts.New(ses)
	identity, _ := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	return identity.Account
}

// GetCallerRegion return the account number
func GetCallerRegion(ses *session.Session) *string {
	return ses.Config.Region
}
