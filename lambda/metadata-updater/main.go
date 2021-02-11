package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io/ioutil"
	"log"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/smithy-go"
)

// {
//     "eventVersion": "2.1",
//     "eventSource": "aws:s3",
//     "awsRegion": "ap-northeast-1",
//     "eventTime": "2021-02-10T04:38:28.261Z",
//     "eventName": "ObjectCreated:Put",
//     "userIdentity": {
//         "principalId": "AWS:AIDAIIYTLRUFSSQXNUPU2"
//     },
//     "requestParameters": {
//         "sourceIPAddress": "60.47.1.228"
//     },
//     "responseElements": {
//         "x-amz-id-2": "AgU98Ub9HHSq8p0q3e986ETpenuPJw//FZEceVACCWFDMP9/JXarviV4zQVLHrKB6zCFI29LdGY430Ry5NV4oD+8f4k23Xjd",
//         "x-amz-request-id": "B1FE8CDA6F9702B3"
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
//             "key": "amazonlinux/2/x86_64/h2o-2.2.6-1.amzn2.x86_64.rpm",
//             "size": 2716232,
//             "urlDecodedKey": "amazonlinux/2/x86_64/h2o-2.2.6-1.amzn2.x86_64.rpm",
//             "versionId": "",
//             "eTag": "ee6c2655afcee02c0f32f469be9b3a29",
//             "sequencer": "0060236347394C03FE"
//         }
//     }
// }

var errSkipped = errors.New("updater: file skipped")

type handler struct {
	s3svc        *s3.Client
	downloader   *manager.Downloader
	uploader     *manager.Uploader
	ssmsvc       *ssm.Client
	outputBucket string

	// the parameter path for GPG secret key
	secretParamPath string

	// directory structure
	depth int

	// full paths for tools
	rpm        string
	gpg        string
	createrepo string
	mergerepo  string
}

func newHandler(ctx context.Context) (*handler, error) {
	// configure AWS clients
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	s3svc := s3.NewFromConfig(cfg)
	downloader := manager.NewDownloader(s3svc)
	uploader := manager.NewUploader(s3svc)
	ssmsvc := ssm.NewFromConfig(cfg)

	// lookup executables
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
	mergerepo, err := exec.LookPath("mergerepo")
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
		depth:           3, // $distribution/$releasever/$basearch
		rpm:             rpm,
		gpg:             gpg,
		createrepo:      createrepo,
		mergerepo:       mergerepo,
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
			if errors.Is(err, errSkipped) {
				continue
			}
			return err
		}
		if err := c.signRPM(ctx, name); err != nil {
			return err
		}
	}

	repos, err := c.listRepos(ctx)
	if err != nil {
		return err
	}

	for _, repo := range repos {
		if err := c.createrepo(ctx, repo); err != nil {
			return err
		}
		if err := c.downloadMetadata(ctx, repo); err != nil {
			return err
		}
		if err := c.mergerepo(ctx, repo); err != nil {
			return err
		}
		if err := c.uploadRPM(ctx, repo); err != nil {
			return err
		}
		if err := c.uploadMetadata(ctx, repo); err != nil {
			return err
		}
	}

	return nil
}

func (c *myContext) Cleanup() {
	os.RemoveAll(c.dir)
}

func (c *myContext) configureGPG(ctx context.Context) error {
	// TODO: make GPG name configureable
	err := ioutil.WriteFile(filepath.Join(c.home, ".rpmmacros"), []byte(`%_signature gpg
%_gpg_name Ichinose Shogo <shogo82148@gmail.com>
%_tmppath /tmp
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
	data, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	log.Println(string(data))

	name := filepath.Join(c.input, filepath.FromSlash(record.S3.Object.URLDecodedKey))
	ext := filepath.Ext(name)
	if ext != ".rpm" {
		return "", errSkipped
	}

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

func (c myContext) listRepos(ctx context.Context) ([]string, error) {
	dir := c.input
	var repos []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		dirs := strings.Split(filepath.ToSlash(rel), "/")
		if len(dirs) == c.handler.depth {
			repos = append(repos, rel)
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return repos, nil
}

type repomd struct {
	Revision string `xml:"revision"`
	Data     []data `xml:"data"`
}

type data struct {
	Type      string   `xml:"type,attr"`
	Timestamp int64    `xml:"timestamp"`
	Location  location `xml:"location"`
	Size      int64    `xml:"size"`
	OpenSize  int64    `xml:"open-size"`
}

type location struct {
	Href string `xml:"href,attr"`
}

func (c myContext) downloadMetadata(ctx context.Context, repo string) error {
	log.Printf("download metadata for %s", repo)

	path := filepath.Join(c.base, repo, "repodata", "repomd.xml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	key := filepath.ToSlash(filepath.Join(repo, "repodata", "repomd.xml"))
	log.Printf("download %s from %s", key, c.handler.outputBucket)
	_, err = c.handler.downloader.Download(ctx, f, &s3.GetObjectInput{
		Bucket: aws.String(c.handler.outputBucket),
		Key:    aws.String(key),
	})
	if err1 := f.Close(); err == nil {
		err = err1
	}
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) && ae.ErrorCode() == "NoSuchKey" {
			// it might be first S3 event.
			// initialize the repository.
			return c.createEmptyRepo(ctx, repo)
		}
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	var metadata repomd
	if err := xml.Unmarshal(data, &metadata); err != nil {
		return err
	}

	for _, data := range metadata.Data {
		log.Println("location: ", data.Location.Href)
		log.Println("size: ", data.Size)
		log.Println("open-size: ", data.OpenSize)
	}

	return nil
}

func (c *myContext) createEmptyRepo(ctx context.Context, repo string) error {
	path := filepath.Join(c.base, repo)
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, c.handler.createrepo, path)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (c *myContext) createrepo(ctx context.Context, repo string) error {
	log.Printf("create repository for %s", repo)
	path := filepath.Join(c.input, repo)
	cmd := exec.CommandContext(ctx, c.handler.createrepo, path)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (c *myContext) mergerepo(ctx context.Context, repo string) error {
	log.Printf("merge repository for %s", repo)
	repo1 := filepath.Join(c.base, repo)
	repo2 := filepath.Join(c.input, repo)
	out := filepath.Join(c.output, repo)
	cmd := exec.CommandContext(
		ctx, c.handler.mergerepo, "--repo", repo1, "--repo", repo2, "--database", "--outputdir", out,
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (c *myContext) uploadMetadata(ctx context.Context, repo string) error {
	log.Printf("upload metadata for %s", repo)
	dir := c.output
	root := filepath.Join(c.output, repo, "repodata")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// skip directories
		if info.IsDir() {
			return nil
		}

		// we need to upload repomd.xml finally
		if info.Name() == "repomd.xml" {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		key := filepath.ToSlash(rel)
		ext := filepath.Ext(path)
		log.Printf("uploading %s to %s", key, c.handler.outputBucket)
		_, err = c.handler.uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(c.handler.outputBucket),
			Key:         aws.String(key),
			ACL:         s3types.ObjectCannedACLPublicRead,
			ContentType: aws.String(mime.TypeByExtension(ext)),
			Body:        f,
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// upload repomd.xml here
	path := filepath.Join(c.output, repo, "repodata", "repomd.xml")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	key := filepath.ToSlash(filepath.Join(repo, "repodata", "repomd.xml"))
	ext := filepath.Ext(".xml")
	log.Printf("uploading %s to %s", key, c.handler.outputBucket)
	_, err = c.handler.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.handler.outputBucket),
		Key:         aws.String(key),
		ACL:         s3types.ObjectCannedACLPublicRead,
		ContentType: aws.String(mime.TypeByExtension(ext)),
		Body:        f,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *myContext) uploadRPM(ctx context.Context, repo string) error {
	dir := c.input
	root := filepath.Join(dir, repo)
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// skip directories
		if info.IsDir() {
			return nil
		}

		// we accepts .rpm only
		ext := filepath.Ext(path)
		if ext != ".rpm" {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		key := filepath.ToSlash(rel)
		log.Printf("uploading %s to %s", key, c.handler.outputBucket)
		_, err = c.handler.uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(c.handler.outputBucket),
			Key:         aws.String(key),
			ACL:         s3types.ObjectCannedACLPublicRead,
			ContentType: aws.String(mime.TypeByExtension(ext)),
			Body:        f,
		})
		if err != nil {
			return err
		}
		return nil
	})
}

func main() {
	h, err := newHandler(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	lambda.Start(h.handleEvent)
}
