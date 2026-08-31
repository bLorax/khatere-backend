package s3

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Connect builds an S3-compatible client and makes sure bucket exists,
// creating it if not.
//
// If accessKey is empty, this falls back to the IAM credential chain
// (EC2/ECS/EKS instance role) instead of static keys — the safer choice
// in production if the app already runs on AWS infrastructure with a
// role attached. Pass accessKey/secretKey explicitly for local Minio,
// or for any provider without IAM-role-based auth (R2, Backblaze, etc).
func Connect(endpoint, region, accessKey, secretKey, bucket string, useSSL bool) (*minio.Client, error) {
	var creds *credentials.Credentials
	if accessKey == "" {
		creds = credentials.NewIAM("")
	} else {
		creds = credentials.NewStaticV4(accessKey, secretKey, "")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  creds,
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("create object storage client: %w", err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region}); err != nil {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
	}

	return client, nil
}
