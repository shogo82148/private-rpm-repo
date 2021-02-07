package main

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var s3svc *s3.Client
var downloader *manager.Downloader
var uploader *manager.Uploader
var outputBucket string

// {
//     "eventVersion": "2.1",
//     "eventSource": "aws:s3",
//     "awsRegion": "ap-northeast-1",
//     "eventTime": "2021-02-07T14:03:05.055Z",
//     "eventName": "ObjectCreated:Put",
//     "userIdentity": {
//         "principalId": "AWS:AIDAIIYTLRUFSSQXNUPU2"
//     },
//     "requestParameters": {
//         "sourceIPAddress": "60.47.1.228"
//     },
//     "responseElements": {
//         "x-amz-id-2": "RY9/nKt11cMK0f/dPBg0keP3VyR9zNRw7Jy2y9GxDOY0gbBN2EbwuU1S9vn0OacyBChqvJLmP7+xNEzu5J6UPsH/bqZtgJeK",
//         "x-amz-request-id": "BE14F554EE9611DA"
//     },
//     "s3": {
//         "s3SchemaVersion": "1.0",
//         "configurationId": "07c783f7-9685-4895-8663-dc19d4bc7682",
//         "bucket": {
//             "name": "shogo82148-rpm-temporary",
//             "ownerIdentity": {
//                 "principalId": "AZFL1NT9HQXA8"
//             },
//             "arn": "arn:aws:s3:::shogo82148-rpm-temporary"
//         },
//         "object": {
//             "key": "h2o-2.2.5-2.amzn2.x86_64.rpm",
//             "size": 2747616,
//             "urlDecodedKey": "h2o-2.2.5-2.amzn2.x86_64.rpm",
//             "versionId": "",
//             "eTag": "b3bbd851f65c03c33a8552ce6a080f3c",
//             "sequencer": "00601FF31C767E02C8"
//         }
//     }
// }
func handleEvent(ctx context.Context, event events.S3Event) (string, error) {
	dir, err := ioutil.TempDir("/tmp/", "updater-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir) // clean up

	repo := filepath.Join(dir, "repo")
	home := filepath.Join(dir, "home")

	err = ioutil.WriteFile(filepath.Join(home, ".rpmmacros"), []byte(`%_signature gpg
%_gpg_name Ichinose Shogo <shogo82148@gmail.com>
`), 0600)
	if err != nil {
		return "", err
	}

	rpm, err := exec.LookPath("rpm")
	if err != nil {
		return "", err
	}
	for _, record := range event.Records {
		name, err := download(ctx, repo, record)
		if err != nil {
			return "", err
		}
		cmd := exec.CommandContext(ctx, rpm, "--addsign", name)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		cmd.Env = []string{
			"HOME=" + home,
		}
		if err := cmd.Run(); err != nil {
			return "", err
		}
	}

	createrepo, err := exec.LookPath("createrepo")
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, createrepo, repo)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	err = filepath.Walk(repo, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket: aws.String(outputBucket),
			Key:    aws.String(filepath.ToSlash(rel)),
			Body:   f,
		})
		if err != nil {
			return err
		}
		return nil
	})

	return "Hello ƛ!", nil
}

func download(ctx context.Context, dir string, record events.S3EventRecord) (string, error) {
	name := filepath.Join(dir, filepath.FromSlash(record.S3.Object.URLDecodedKey))
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return "", err
	}

	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}

	_, err = downloader.Download(ctx, f, &s3.GetObjectInput{
		Bucket: aws.String(record.S3.Bucket.Name),
		Key:    aws.String(record.S3.Object.Key),
	})
	if err1 := f.Close(); err == nil {
		err = err1
	}
	if err != nil {
		return "", err
	}
	return name, nil
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}
	s3svc = s3.NewFromConfig(cfg)
	downloader = manager.NewDownloader(s3svc)
	uploader = manager.NewUploader(s3svc)
	outputBucket = os.Getenv("OUTPUT_BUCKET")

	lambda.Start(handleEvent)
}
