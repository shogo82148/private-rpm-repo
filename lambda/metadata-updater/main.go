package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/smithy-go"
	"github.com/shogo82148/go-retry"
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
	outputBucket string
	lockTable    string

	// the parameter path for GPG secret key
	secretParamPath string

	// directory structure
	depth int

	// Clients for aws services
	s3svc       *s3.Client
	downloader  *manager.Downloader
	uploader    *manager.Uploader
	ssmsvc      *ssm.Client
	dynamodbsvc *dynamodb.Client

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
	dynamodbsvc := dynamodb.NewFromConfig(cfg)

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
		outputBucket:    os.Getenv("OUTPUT_BUCKET"),
		secretParamPath: os.Getenv("GPG_SECRET_KEY"),
		depth:           3, // $distribution/$releasever/$basearch
		lockTable:       os.Getenv("LOCKER_TABLE"),

		s3svc:       s3svc,
		downloader:  downloader,
		uploader:    uploader,
		ssmsvc:      ssmsvc,
		dynamodbsvc: dynamodbsvc,
		rpm:         rpm,
		gpg:         gpg,
		createrepo:  createrepo,
		mergerepo:   mergerepo,
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
	dir, err := os.MkdirTemp("/tmp/", "updater-")
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
		if err := c.handleRepo(ctx, repo); err != nil {
			return err
		}
	}

	return nil
}

func (c *myContext) handleRepo(ctx context.Context, repo string) error {
	if err := c.lockRepo(ctx, repo); err != nil {
		return err
	}
	defer c.unlockRepo(ctx, repo)

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
	return nil
}

func (c *myContext) Cleanup() {
	os.RemoveAll(c.dir)
}

func (c *myContext) configureGPG(ctx context.Context) error {
	if err := c.createGPGConfig(ctx); err != nil {
		return err
	}

	if err := c.importGPGSecret(ctx); err != nil {
		return fmt.Errorf("failed to import GPG secret: %w", err)
	}

	uid, err := c.getUserID(ctx)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(c.home, ".rpmmacros"), []byte(`%_signature gpg
%_gpg_name `+uid+`
%_tmppath /tmp
`), 0600)
	if err != nil {
		return err
	}
	return nil
}

func (c *myContext) createGPGConfig(ctx context.Context) error {
	dir := filepath.Join(c.home, ".gnupg")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create ~/.gnupg: %w", err)
	}

	conf := filepath.Join(dir, "gpg.conf")
	err := os.WriteFile(conf, []byte(`
# ref. https://ngkz.github.io/2020/01/gpg-hardening/
# use SHA-512 when signing a key
cert-digest-algo SHA512
# override recipient key cipher preferences
# remove 3DES and prefer AES256
personal-cipher-preferences AES256 AES192 AES CAST5
# override recipient key digest preferences
# remove SHA-1 and prefer SHA-512
personal-digest-preferences SHA512 SHA384 SHA256 SHA224
# remove SHA-1 and 3DES from cipher preferences of newly created key
default-preference-list SHA512 SHA384 SHA256 SHA224 AES256 AES192 AES CAST5 ZLIB BZIP2 ZIP Uncompressed
# reject SHA-1 signature
weak-digest SHA1
# never allow use 3DES
disable-cipher-algo 3DES
# use AES256 when symmetric encryption
s2k-cipher-algo AES256
# use SHA-512 when symmetric encryption
s2k-digest-algo SHA512
# mangle password many times as possible when symmetric encryption
s2k-count 65011712
`), 0600)
	if err != nil {
		return fmt.Errorf("failed to create ~/.gnupg/gpg.conf: %w", err)
	}
	return nil
}

func (c *myContext) importGPGSecret(ctx context.Context) error {
	out, err := c.handler.ssmsvc.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(c.handler.secretParamPath),
		WithDecryption: true,
	})
	if err != nil {
		return err
	}

	key := filepath.Join(c.home, "secret.asc")
	err = os.WriteFile(key, []byte(aws.ToString(out.Parameter.Value)), 0600)
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

func (c *myContext) getUserID(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, c.handler.gpg, "--with-colons", "--list-keys")
	cmd.Env = []string{
		"HOME=" + c.home,
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		col := strings.Split(line, ":")
		// ref. https://github.com/gpg/gnupg/blob/master/doc/DETAILS
		if len(col) < 10 {
			continue
		}
		if col[0] == "uid" {
			return unescape(col[9]), nil
		}
	}
	return "", errors.New("updater: user id not found")
}

func unescape(str string) string {
	var buf strings.Builder
	buf.Grow(len(str))
	for i := 0; i < len(str); i++ {
		if str[i] == '\\' {
			i++
			switch str[i] {
			case 'a':
				buf.WriteRune('\a')
			case 'b':
				buf.WriteRune('\b')
			case 'f':
				buf.WriteRune('\f')
			case 'n':
				buf.WriteRune('\n')
			case 'r':
				buf.WriteRune('\r')
			case 't':
				buf.WriteRune('\t')
			case 'v':
				buf.WriteRune('\v')
			case '\\':
				buf.WriteRune('\\')
			case '\'':
				buf.WriteRune('\'')
			case '"':
				buf.WriteRune('"')
			case '?':
				buf.WriteRune('?')
			case '0', '1', '2', '3':
				if i+3 > len(str) {
					break
				}
				ch, err := strconv.ParseInt(str[i:i+3], 8, 8)
				if err != nil {
					break
				}
				i += 2
				buf.WriteByte(byte(ch))
			case 'x', 'X':
				if i+3 > len(str) {
					break
				}
				ch, err := strconv.ParseInt(str[i+1:i+3], 16, 8)
				if err != nil {
					break
				}
				i += 2
				buf.WriteByte(byte(ch))
			}
			continue
		}
		buf.WriteByte(str[i])
	}
	return buf.String()
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

var policy = retry.Policy{
	MinDelay: 100 * time.Millisecond,
	MaxDelay: 30 * time.Second,
	Jitter:   time.Second,
}

func (c *myContext) lockRepo(ctx context.Context, repo string) error {
	lc, _ := lambdacontext.FromContext(ctx)
	reqID := lc.AwsRequestID
	retrier := policy.Start(ctx)
	for retrier.Continue() {
		_, err := c.handler.dynamodbsvc.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(c.handler.lockTable),
			Item: map[string]dbtypes.AttributeValue{
				"id":         &dbtypes.AttributeValueMemberS{Value: repo},
				"request_id": &dbtypes.AttributeValueMemberS{Value: reqID},
			},
			ConditionExpression: aws.String("attribute_not_exists(id)"),
		})
		if err == nil {
			return nil
		}
	}
	return errors.New("failed to lock")
}

func (c *myContext) unlockRepo(ctx context.Context, repo string) error {
	_, err := c.handler.dynamodbsvc.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(c.handler.lockTable),
		Key: map[string]dbtypes.AttributeValue{
			"id": &dbtypes.AttributeValueMemberS{Value: repo},
		},
	})
	return err
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

func (c *myContext) downloadMetadata(ctx context.Context, repo string) error {
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

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var metadata repomd
	if err := xml.Unmarshal(data, &metadata); err != nil {
		return err
	}

	for _, data := range metadata.Data {
		path := filepath.Join(c.base, repo, filepath.FromSlash(data.Location.Href))
		key := filepath.ToSlash(filepath.Join(repo, filepath.FromSlash(data.Location.Href)))
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		log.Printf("download %s from %s", key, c.handler.outputBucket)
		_, err = c.handler.downloader.Download(ctx, f, &s3.GetObjectInput{
			Bucket: aws.String(c.handler.outputBucket),
			Key:    aws.String(key),
		})
		if err1 := f.Close(); err == nil {
			err = err1
		}
		if err != nil {
			return err
		}
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
	repo1 := filepath.Join(c.input, repo)
	repo2 := filepath.Join(c.base, repo)
	out := filepath.Join(c.output, repo)
	cmd := exec.CommandContext(
		ctx, c.handler.mergerepo, "--database", "--omit-baseurl", "--all", "--repo", repo1, "--repo", repo2, "--outputdir", out,
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
