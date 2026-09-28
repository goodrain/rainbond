package storage

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/s3"
)

var chunkSessionIdentity = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func (s3s *S3Storage) cleanupChunkObjects(sessionID string) error {
	if !chunkSessionIdentity.MatchString(sessionID) || s3s.s3Client == nil {
		return fmt.Errorf("invalid upload chunk cleanup scope")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const bucket = "grdata"
	prefix := s3s.GetChunkDir(sessionID) + "/"
	objects := []*s3.ObjectIdentifier{}
	keys := map[string]bool{}
	cursors := map[string]bool{}
	cursor := ""
	for page := 0; ; page++ {
		if page >= 100 {
			return fmt.Errorf("upload chunk inventory limit exceeded")
		}
		input := &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String(prefix), MaxKeys: aws.Int64(1000)}
		if cursor != "" {
			input.ContinuationToken = aws.String(cursor)
		}
		result, err := s3s.s3Client.ListObjectsV2WithContext(ctx, input)
		if err != nil {
			return fmt.Errorf("upload chunk inventory unavailable")
		}
		if result == nil || result.IsTruncated == nil || aws.StringValue(result.Name) != bucket || aws.StringValue(result.Prefix) != prefix {
			return fmt.Errorf("upload chunk inventory scope unverified")
		}
		for _, object := range result.Contents {
			if object == nil || object.Key == nil || !strings.HasPrefix(*object.Key, prefix) || keys[*object.Key] || len(objects) >= 10000 {
				return fmt.Errorf("invalid upload chunk inventory")
			}
			keys[*object.Key] = true
			objects = append(objects, &s3.ObjectIdentifier{Key: object.Key})
		}
		if !*result.IsTruncated {
			break
		}
		cursor = aws.StringValue(result.NextContinuationToken)
		if cursor == "" || cursors[cursor] {
			return fmt.Errorf("invalid upload chunk pagination")
		}
		cursors[cursor] = true
	}
	if len(objects) == 0 {
		return nil
	}
	for start := 0; start < len(objects); start += 1000 {
		end := start + 1000
		if end > len(objects) {
			end = len(objects)
		}
		batch := objects[start:end]
		request, result := s3s.s3Client.DeleteObjectsRequest(&s3.DeleteObjectsInput{Bucket: aws.String(bucket), Delete: &s3.Delete{Objects: batch, Quiet: aws.Bool(false)}})
		// A lost response is not permission to replay a destructive request.
		request.Retryer = awsclient.DefaultRetryer{NumMaxRetries: 0}
		request.SetContext(ctx)
		if err := request.Send(); err != nil {
			return fmt.Errorf("upload chunk deletion unconfirmed")
		}
		if result == nil || len(result.Errors) > 0 || len(result.Deleted) != len(batch) {
			return fmt.Errorf("upload chunks were not completely deleted")
		}
		remaining := map[string]bool{}
		for _, object := range batch {
			remaining[*object.Key] = true
		}
		for _, deleted := range result.Deleted {
			if deleted == nil || deleted.Key == nil || !remaining[*deleted.Key] {
				return fmt.Errorf("invalid upload chunk deletion acknowledgement")
			}
			delete(remaining, *deleted.Key)
		}
		if len(remaining) != 0 {
			return fmt.Errorf("missing upload chunk deletion acknowledgement")
		}
	}
	final, err := s3s.s3Client.ListObjectsV2WithContext(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String(prefix), MaxKeys: aws.Int64(1)})
	if err != nil || final == nil || final.IsTruncated == nil || *final.IsTruncated || len(final.Contents) != 0 || aws.StringValue(final.Name) != bucket || aws.StringValue(final.Prefix) != prefix {
		return fmt.Errorf("upload chunk absence unconfirmed")
	}
	return nil
}
