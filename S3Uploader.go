package main

import (
	"context"
	"errors"
	"fmt"
	b2b "github.com/kurin/blazer/b2"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func S3Upload(ctx context.Context, cancel context.CancelFunc, filesToUpload chan FileToUpload) {
	defer cancel()
	bucket, err := openBucket()
	if err != nil {
		log.Errorf("Failed to open bucket on backblaze: %s", err)
		return
	} else {
		for {
			select {
			case <-ctx.Done():
				return
			case fileToUpload := <-filesToUpload:
				log.Infof("Uploading new file: %s", fileToUpload.filePathName)
				uploadFile(fileToUpload, bucket)
			}
		}
	}
}

func openBucket() (*b2b.Bucket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	b2, err := b2b.NewClient(ctx, backblazeAccount, backblazePassword)
	if err != nil {
		return nil, errors.New("Failed to connect to Backblaze B2 Cloud Storage: " + err.Error())
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	bucket, err := b2.Bucket(ctx, backblazeBucketName)
	if err != nil {
		return nil, errors.New("Failed to open bucket: " + err.Error())
	}
	return bucket, nil
}

func uploadFile(fileToUpload FileToUpload, bucket *b2b.Bucket) {
	reader, err := os.Open(fileToUpload.filePathName)
	if err != nil {
		log.Errorf("Failed to open file: %s", err)
	} else {
		defer reader.Close()
		name := bucketPrefix + strconv.Itoa(fileToUpload.fileTime.Year()) + "-" +
			fmt.Sprintf("%02d", fileToUpload.fileTime.Month()) + "-" +
			fmt.Sprintf("%02d", fileToUpload.fileTime.Day()) + "/" + filepath.Base(fileToUpload.filePathName)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()
		writer := bucket.Object(name).NewWriter(ctx)
		defer writer.Close()
		if _, err := io.Copy(writer, reader); err != nil {
			log.Errorf("Failed to upload file to backblaze: %s", err)
		}
		log.Infof("Successfully uploaded file to backblaze: %s", name)
	}
}
