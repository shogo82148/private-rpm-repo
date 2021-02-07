package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

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

type handler struct {
	s3svc        *s3.Client
	downloader   *manager.Downloader
	uploader     *manager.Uploader
	ssmsvc       *ssm.Client
	outputBucket string

	// the parameter path for GPG secret key
	secretParamPath string

	// full paths for tools
	rpm        string
	gpg        string
	createrepo string
}

func newHandler(ctx context.Context) (*handler, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	s3svc := s3.NewFromConfig(cfg)
	downloader := manager.NewDownloader(s3svc)
	uploader := manager.NewUploader(s3svc)
	ssmsvc := ssm.NewFromConfig(cfg)

	rpm, err := exec.LookPath("rpm")
	if err != nil {
		return nil, err
	}
	gpg, err := exec.LookPath("gpg")
	if err != nil {
		return nil, err
	}
	createrepo, err := exec.LookPath("createrepo")
	if err != nil {
		return nil, err
	}

	return &handler{
		s3svc:           s3svc,
		downloader:      downloader,
		uploader:        uploader,
		ssmsvc:          ssmsvc,
		outputBucket:    os.Getenv("OUTPUT_BUCKET"),
		secretParamPath: os.Getenv("GPG_SECRET_KEY"),
		rpm:             rpm,
		gpg:             gpg,
		createrepo:      createrepo,
	}, nil
}

func (h *handler) handleEvent(ctx context.Context, event events.S3Event) error {
	c, err := h.newContext(event)
	if err != nil {
		log.Println(err)
		return err
	}
	defer c.Cleanup()
	if err := c.handle(ctx); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (h *handler) newContext(event events.S3Event) (*myContext, error) {
	dir, err := ioutil.TempDir("/tmp/", "updater-")
	if err != nil {
		return nil, err
	}

	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0700); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	base := filepath.Join(dir, "base")
	if err := os.MkdirAll(base, 0700); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	input := filepath.Join(dir, "input")
	if err := os.MkdirAll(input, 0700); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	output := filepath.Join(dir, "output")
	if err := os.MkdirAll(output, 0700); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	return &myContext{
		handler: h,
		event:   event,
		dir:     dir,
		home:    home,
		base:    base,
		input:   input,
		output:  output,
	}, nil
}

type myContext struct {
	handler *handler

	event events.S3Event

	// temporary directory
	dir string

	// dummy home directory
	home string

	// old repository metadata
	base string

	// for new rpm files
	input string

	// new repository metadata
	output string
}

func (c *myContext) handle(ctx context.Context) error {
	if err := c.configureGPG(ctx); err != nil {
		return err
	}

	for _, record := range c.event.Records {
		name, err := c.downloadRPM(ctx, record)
		if err != nil {
			return err
		}
		if err := c.signRPM(ctx, name); err != nil {
			return err
		}
	}

	if err := c.createrepo(ctx); err != nil {
		return err
	}

	// err = filepath.Walk(repo, func(path string, info os.FileInfo, err error) error {
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if info.IsDir() {
	// 		return nil
	// 	}
	// 	rel, err := filepath.Rel(repo, path)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	f, err := os.Open(path)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	defer f.Close()
	// 	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
	// 		Bucket: aws.String(outputBucket),
	// 		Key:    aws.String(filepath.ToSlash(rel)),
	// 		Body:   f,
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return nil
	// })

	return nil
}

func (c *myContext) Cleanup() {
	os.RemoveAll(c.dir)
}

func (c *myContext) configureGPG(ctx context.Context) error {
	// TODO: make GPG name configureable
	err := ioutil.WriteFile(filepath.Join(c.home, ".rpmmacros"), []byte(`%_signature gpg
%_gpg_name Ichinose Shogo <shogo82148@gmail.com>
`), 0600)
	if err != nil {
		return err
	}

	out, err := c.handler.ssmsvc.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String("/" + c.handler.secretParamPath),
		WithDecryption: true,
	})
	if err != nil {
		return err
	}

	key := filepath.Join(c.home, "secret.asc")
	err = ioutil.WriteFile(key, []byte(aws.ToString(out.Parameter.Value)), 0600)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, c.handler.gpg, "--import", key)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Env = []string{
		"HOME=" + c.home,
		"TMPDIR=/tmp",
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (c *myContext) signRPM(ctx context.Context, name string) error {
	log.Printf("signing %s", name)

	cmd := exec.CommandContext(ctx, c.handler.rpm, "--addsign", name)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Env = []string{
		"HOME=" + c.home,
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// download rpm files into the input directory
func (c *myContext) downloadRPM(ctx context.Context, record events.S3EventRecord) (string, error) {
	name := filepath.Join(c.input, filepath.FromSlash(record.S3.Object.URLDecodedKey))
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return "", err
	}

	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}

	log.Printf("downloading %s from %s", record.S3.Object.Key, record.S3.Bucket.Name)
	_, err = c.handler.downloader.Download(ctx, f, &s3.GetObjectInput{
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

func (c *myContext) createrepo(ctx context.Context) error {
	log.Print("create repository")
	cmd := exec.CommandContext(ctx, c.handler.createrepo, c.input)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func main() {
	h, err := newHandler(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	lambda.Start(h.handleEvent)
}
