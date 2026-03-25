package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	r2ClientOnce sync.Once
	r2Client     *s3.Client
	r2ClientErr  error
)

type packStatusResponse struct {
	Status         string `json:"status"`
	URL            string `json:"url,omitempty"`
	Key            string `json:"key,omitempty"`
	PollIntervalMS int    `json:"poll_interval_ms,omitempty"`
	Error          string `json:"error,omitempty"`
}

func packPublicBaseURL() string {
	baseURL := strings.TrimSpace(os.Getenv("PACKS_PUBLIC_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://decryptmypack-packs.restartfu.workers.dev"
	}
	return strings.TrimRight(baseURL, "/")
}

func packPublicURL(objectKey string) string {
	return packPublicBaseURL() + "/" + strings.TrimLeft(objectKey, "/")
}

func r2BucketName() string {
	name := strings.TrimSpace(os.Getenv("R2_BUCKET_NAME"))
	if name == "" {
		name = "decryptmypack"
	}
	return name
}

func r2Endpoint() string {
	endpoint := strings.TrimSpace(os.Getenv("R2_ENDPOINT"))
	if endpoint == "" {
		endpoint = "https://d25fedc31f9a231f730ad862ad909b2b.r2.cloudflarestorage.com"
	}
	return strings.TrimRight(endpoint, "/")
}

func r2ClientInstance(ctx context.Context) (*s3.Client, error) {
	r2ClientOnce.Do(func() {
		accessKeyID := strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID"))
		secretAccessKey := strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY"))
		if accessKeyID == "" || secretAccessKey == "" {
			r2ClientErr = errors.New("missing R2_ACCESS_KEY_ID or R2_SECRET_ACCESS_KEY")
			return
		}

		cfg, err := config.LoadDefaultConfig(
			ctx,
			config.WithRegion("auto"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
		)
		if err != nil {
			r2ClientErr = err
			return
		}

		r2Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(r2Endpoint())
			o.UsePathStyle = true
		})
	})
	return r2Client, r2ClientErr
}

func uploadPackToR2(objectKey string, reader io.Reader) error {
	client, err := r2ClientInstance(context.Background())
	if err != nil {
		return err
	}

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:             aws.String(r2BucketName()),
		Key:                aws.String(objectKey),
		Body:               reader,
		CacheControl:       aws.String("public, max-age=86400"),
		ContentDisposition: aws.String(fmt.Sprintf("attachment; filename=%q", path.Base(objectKey))),
		ContentType:        aws.String("application/zip"),
	})
	return err
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func packObjectKey(target, port string) string {
	target = strings.ToLower(target)
	return path.Join("packs", target, port, target+".zip")
}
