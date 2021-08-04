package storage

import (
	"context"
	"fmt"
	"github.com/onepanelio/cli/files"
	"github.com/onepanelio/cli/util"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	minio "github.com/minio/minio-go/v6"
)

// ArtifactRepositoryS3Provider is meant to be used
// by the CLI. CLI will marshal this struct into the correct
// YAML structure for k8s configmap / secret.
type ArtifactRepositoryS3Provider struct {
	KeyFormat       string `yaml:"keyFormat"`
	Bucket          string
	Endpoint        string
	PublicEndpoint  string `yaml:"publicEndpoint"`
	PublicInsecure  bool   `yaml:"publicInsecure"`
	Insecure        bool
	Region          string                   `yaml:"region,omitempty"`
	AccessKeySecret ArtifactRepositorySecret `yaml:"accessKeySecret"`
	SecretKeySecret ArtifactRepositorySecret `yaml:"secretKeySecret"`
	AccessKey       string                   `yaml:"accessKey,omitempty"`
	Secretkey       string                   `yaml:"secretKey,omitempty"`
}

// ArtifactRepositoryGCSProvider is meant to be used
// by the CLI. CLI will marshal this struct into the correct
// YAML structure for k8s configmap / secret.
type ArtifactRepositoryGCSProvider struct {
	KeyFormat               string `yaml:"keyFormat"`
	Bucket                  string
	Endpoint                string
	Insecure                bool
	ServiceAccountKey       string                   `yaml:"serviceAccountKey,omitempty"`
	ServiceAccountKeySecret ArtifactRepositorySecret `yaml:"serviceAccountKeySecret"`
	ServiceAccountJSON      string                   `yaml:"serviceAccountJSON,omitempty"`
}

// ArtifactRepositoryABSProvider - Azure Blob Storage is meant to be used
// by the CLI. CLI will marshal this struct into the correct
// YAML structure for k8s configmap / secret.
type ArtifactRepositoryABSProvider struct {
	KeyFormat          string `yaml:"keyFormat"`
	Bucket             string
	Endpoint           string
	Insecure           bool
	AccessKeySecret    ArtifactRepositorySecret `yaml:"accessKeySecret"`
	SecretKeySecret    ArtifactRepositorySecret `yaml:"secretKeySecret"`
	AccessKey          string                   `yaml:"accessKey,omitempty"`
	Secretkey          string                   `yaml:"secretKey,omitempty"`
	Container          string                   `yaml:"container"`
	StorageAccountKey  string                   `yaml:"storageAccountKey"`
	StorageAccountName string                   `yaml:"storageAccountName"`
}

// ArtifactRepositoryProvider is used to setup access into AWS Cloud Storage
// or Google Cloud storage.
// - The relevant sub-struct (S3, GCS) is unmarshalled into from the cluster configmap.
// Right now, either the S3 or GCS struct will be filled in. Multiple cloud
// providers are not supported at the same time in params.yaml (manifests deployment).
type ArtifactRepositoryProvider struct {
	S3  *ArtifactRepositoryS3Provider  `yaml:"s3,omitempty"`
	GCS *ArtifactRepositoryGCSProvider `yaml:"gcs,omitempty"`
	ABS *ArtifactRepositoryABSProvider `yaml:"abs,omitempty"`
}

// Load loads any provider specific information required from the cluster
func (a *ArtifactRepositoryProvider) Load(c *kubernetes.Clientset, namespace string) error {
	if a.GCS == nil {
		return nil
	}

	secret, err := c.CoreV1().Secrets(namespace).Get(context.Background(), "onepanel", v1.GetOptions{})
	if err != nil {
		return err
	}

	if secretKeyBytes, ok := secret.Data["artifactRepositoryS3SecretKey"]; ok {
		a.GCS.ServiceAccountKeySecret.Key = string(secretKeyBytes)

		return nil
	}

	return fmt.Errorf("unable to read artifact configuration")
}

// Endpoint returns the Endpoint of the currently set Provider
func (a *ArtifactRepositoryProvider) Endpoint() (string, error) {
	if a.S3 != nil {
		return a.S3.Endpoint, nil
	}
	if a.GCS != nil {
		return a.GCS.Endpoint, nil
	}
	if a.ABS != nil {
		return a.ABS.Endpoint, nil
	}

	return "", fmt.Errorf("provider not set")
}

// PublicEndpoint returns the Publicly accessible endpoint of the currently set Provider
func (a *ArtifactRepositoryProvider) PublicEndpoint(namespace, domain string) (string, error) {
	if a.S3 != nil {
		return a.S3.PublicEndpoint, nil
	}
	if a.GCS != nil || a.ABS != nil {
		return fmt.Sprintf("sys-storage-%v.%v", namespace, domain), nil
	}

	return "", fmt.Errorf("provider not set")
}

// AccessKey returns the AccessKey of the currently set Provider
func (a *ArtifactRepositoryProvider) AccessKey() (string, error) {
	if a.S3 != nil {
		return a.S3.AccessKey, nil
	}
	if a.GCS != nil {
		return a.GCS.Bucket, nil
	}
	if a.ABS != nil {
		return a.ABS.StorageAccountName, nil
	}

	return "", fmt.Errorf("provider not set")
}

// AccessSecret returns the AccessSecret of the currently set Provider
func (a *ArtifactRepositoryProvider) AccessSecret() (string, error) {
	if a.S3 != nil {
		return a.S3.Secretkey, nil
	}
	if a.GCS != nil {
		return a.GCS.ServiceAccountKeySecret.Key, nil
	}
	if a.ABS != nil {
		return a.ABS.StorageAccountKey, nil
	}

	return "", fmt.Errorf("provider not set")
}

// Bucket returns the name of the bucket of the currently set Provider
func (a *ArtifactRepositoryProvider) Bucket() (string, error) {
	if a.S3 != nil {
		return a.S3.Bucket, nil
	}
	if a.GCS != nil {
		return a.GCS.Bucket, nil
	}
	if a.ABS != nil {
		return a.ABS.Container, nil
	}

	return "", fmt.Errorf("provider not set")
}

// MinioClient creates a Minio client using the currently set Provider
func (a *ArtifactRepositoryProvider) MinioClient(namespace, domain string, useSsl bool) (*minio.Client, error) {
	endpoint, err := a.PublicEndpoint(namespace, domain)
	if err != nil {
		return nil, err
	}

	accessKeyName, err := a.AccessKey()
	if err != nil {
		return nil, err
	}

	accessKeySecret, err := a.AccessSecret()
	if err != nil {
		return nil, err
	}

	var minioClient *minio.Client

	if a.S3 != nil {
		minioClient, err = minio.NewWithRegion(endpoint, accessKeyName, accessKeySecret, useSsl, a.S3.Region)
		if err != nil {
			return nil, err
		}
	} else {
		minioClient, err = minio.New(endpoint, accessKeyName, accessKeySecret, useSsl)
		if err != nil {
			return nil, err
		}
	}

	return minioClient, nil
}

// ArtifactRepositorySecret holds information about a kubernetes Secret.
// - The "key" is the specific key inside the Secret.
// - The "name" is the name of the Secret.
// Usually, this is used to figure out what secret to look into for a specific value.
type ArtifactRepositorySecret struct {
	Key  string `yaml:"key"`
	Name string `yaml:"name"`
}

// MarshalToYaml is used by the CLI to generate configmaps during deployment
// or build operations.
func (a *ArtifactRepositoryS3Provider) MarshalToYaml() (string, error) {
	builder := &strings.Builder{}
	encoder := yaml.NewEncoder(builder)
	encoder.SetIndent(6)
	defer encoder.Close()
	err := encoder.Encode(&ArtifactRepositoryProvider{
		S3: &ArtifactRepositoryS3Provider{
			KeyFormat:      a.KeyFormat,
			Bucket:         a.Bucket,
			Endpoint:       a.Endpoint,
			PublicEndpoint: a.PublicEndpoint,
			PublicInsecure: a.PublicInsecure,
			Insecure:       a.Insecure,
			Region:         a.Region,
			AccessKeySecret: ArtifactRepositorySecret{
				Name: a.AccessKeySecret.Name,
				Key:  a.AccessKeySecret.Key,
			},
			SecretKeySecret: ArtifactRepositorySecret{
				Name: a.SecretKeySecret.Name,
				Key:  a.SecretKeySecret.Key,
			},
		},
	})

	if err != nil {
		return "", err
	}

	return builder.String(), nil
}

// MarshalToYaml is used by the CLI to generate configmaps during deployment
// or build operations.
func (g *ArtifactRepositoryGCSProvider) MarshalToYaml() (string, error) {
	builder := &strings.Builder{}
	encoder := yaml.NewEncoder(builder)
	encoder.SetIndent(6)
	defer encoder.Close()
	err := encoder.Encode(&ArtifactRepositoryProvider{
		GCS: &ArtifactRepositoryGCSProvider{
			KeyFormat: g.KeyFormat,
			Bucket:    g.Bucket,
			Endpoint:  g.Endpoint,
			Insecure:  g.Insecure,
			ServiceAccountKeySecret: ArtifactRepositorySecret{
				Key:  "artifactRepositoryGCSServiceAccountKey",
				Name: "onepanel",
			},
		},
	})

	if err != nil {
		return "", err
	}

	return builder.String(), nil
}

// FormatKey replaces placeholder values with their actual values and returns this string.
// {{workflow.namespace}} -> namespace
// {{workflow.name}} -> workflowName
// {{pod.name}} -> podName
func (a *ArtifactRepositoryS3Provider) FormatKey(namespace, workflowName, podName string) string {
	keyFormat := a.KeyFormat

	keyFormat = strings.Replace(keyFormat, "{{workflow.namespace}}", namespace, -1)
	keyFormat = strings.Replace(keyFormat, "{{workflow.name}}", workflowName, -1)
	keyFormat = strings.Replace(keyFormat, "{{pod.name}}", podName, -1)

	return keyFormat
}

// FormatKey replaces placeholder values with their actual values and returns this string.
// {{workflow.namespace}} -> namespace
// {{workflow.name}} -> workflowName
// {{pod.name}} -> podName
func (g *ArtifactRepositoryGCSProvider) FormatKey(namespace, workflowName, podName string) string {
	keyFormat := g.KeyFormat

	keyFormat = strings.Replace(keyFormat, "{{workflow.namespace}}", namespace, -1)
	keyFormat = strings.Replace(keyFormat, "{{workflow.name}}", workflowName, -1)
	keyFormat = strings.Replace(keyFormat, "{{pod.name}}", podName, -1)

	return keyFormat
}

// TestMinioStorageConnection checks to see if the storage connection has all of the requirements for Onepanel
// This includes connecting, creating a file, downloading a file, deleting a file.
// An error with a human friendly message is returned, if there is one.
func TestMinioStorageConnection(client *minio.Client, bucketName string) error {
	exists, err := client.BucketExists(bucketName)
	if err != nil {
		if minioErr, ok := err.(minio.ErrorResponse); ok && minioErr.Code == "SignatureDoesNotMatch" {
			return fmt.Errorf("unable to connect to the bucket with provided credentials. Original error: %v", err.Error())
		}

		return err
	}
	if !exists {
		return fmt.Errorf("Bucket '%v' does not exist", bucketName)
	}

	localFilePath := filepath.Join(".onepanel", "test.txt")
	if err := files.CreateIfNotExist(localFilePath); err != nil {
		return fmt.Errorf("unable to create test file to upload. Original error: %v", err.Error())
	}

	randomString, err := util.RandASCIIString(16)
	if err != nil {
		return err
	}

	objectPath := "onepanel/storage-test/" + randomString + ".txt"

	// Make sure the object does not exist first - we don't want to overwrite anything
	_, err = client.StatObject(bucketName, objectPath, minio.StatObjectOptions{})
	if err != nil {
		// We want NoSuchKey, that means it's ok
		if minioErr, ok := err.(minio.ErrorResponse); ok && minioErr.Code != "NoSuchKey" {
			return err
		}
		err = nil
	} else {
		return fmt.Errorf("test file '%v' already exists, please try again", objectPath)
	}

	_, err = client.FPutObject(bucketName, objectPath, localFilePath, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("unable to upload file object. Original error %v", err)
	}

	if _, err := files.DeleteIfExists(localFilePath); err != nil {
		return fmt.Errorf("unable to delete test file locally. Original error: %v", err.Error())
	}

	err = client.FGetObject(bucketName, objectPath, localFilePath, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("unable to download test file locally. Original error: %v", err.Error())
	}

	err = client.RemoveObject(bucketName, objectPath)
	if err != nil {
		return fmt.Errorf("unanle to delete test file. Original error: %v", err.Error())
	}

	if _, err := files.DeleteIfExists(localFilePath); err != nil {
		return fmt.Errorf("unable to delete test file locally. Original error: %v", err.Error())
	}

	return nil
}
