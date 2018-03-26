package stacks

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/myhelix/terraform-experimental/terracanary/config"

	"errors"
	"fmt"
	"os"
	"sort"
)

var AWSSession *session.Session
var s3Service *s3.S3

func init() {
	//TODO: use Once for creating session instead of init; then read this from config
	os.Setenv("AWS_REGION", "us-east-1")

	AWSSession = session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	cred, err := AWSSession.Config.Credentials.Get()
	if err != nil {
		panic(err)
	}
	s3Service = s3.New(AWSSession)
	// Set up credentials env for terraform, which doesn't understand assume-role config on dev machines
	os.Setenv("AWS_ACCESS_KEY_ID", cred.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", cred.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", cred.SessionToken)
}

// Returns stacks sorted by version, filtered by argument (or "" for any)
// This may include the legacy stack, if no filter is given
func All(subdir string) (stacks []Stack, err error) {
	loi := &s3.ListObjectsInput{
		Bucket: aws.String(config.Global.StateFileBucket),
		Prefix: aws.String(config.Global.StateFileBase),
	}
	resp, err := s3Service.ListObjects(loi)
	if err != nil {
		return nil, fmt.Errorf("Error listing bucket %s: %s", *loi.Bucket, err)
	}
	if *resp.IsTruncated {
		//TODO: Handle this properly
		return nil, errors.New("Can't handle truncated response yet in aws.AllStacks()")
	}
	for _, obj := range resp.Contents {
		name := *obj.Key

		stack, err := fromStateFileName(name)
		if err != nil {
			return nil, err
		}
		stacks = append(stacks, stack)
	}

	if subdir != "" {
		var filtered []Stack
		for _, s := range stacks {
			if s.Subdir == subdir {
				filtered = append(filtered, s)
			}
		}
		stacks = filtered
	}

	sort.Slice(stacks, func(i, j int) bool {
		return stacks[i].Version < stacks[j].Version
	})
	return
}

// This returns the next stack version number available for a given subdir (or overall, if blank)
func Next(subdir string) (uint, error) {
	all, err := All(subdir)
	if err != nil {
		return 0, err
	}
	if len(all) == 0 {
		return 1, nil
	}
	// List from All() is sorted
	return all[len(all)-1].Version + 1, nil
}

func (s Stack) Exists() (bool, error) {
	hoi := &s3.HeadObjectInput{
		Bucket: aws.String(config.Global.StateFileBucket),
		Key:    aws.String(s.stateFileName()),
	}
	_, err := s3Service.HeadObject(hoi)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "NotFound" {
			// No such file
			return false, nil
		} else {
			return false, fmt.Errorf("Error calling HeadObject for '%s': %s", *hoi.Key, err)
		}
	}
	return true, nil
}

func (s Stack) RemoveState() error {
	doi := &s3.DeleteObjectInput{
		Bucket: aws.String(config.Global.StateFileBucket),
		Key:    aws.String(s.stateFileName()),
	}
	_, err := s3Service.DeleteObject(doi)
	if err != nil {
		return fmt.Errorf("Error removing statefile '%s': %s", *doi.Key, err.Error())
	}
	return nil
}
