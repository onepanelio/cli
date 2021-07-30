package storage

import (
	"strings"

	"gopkg.in/yaml.v3"
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
	KeyFormat       string `yaml:"keyFormat"`
	Bucket          string
	Endpoint        string
	Insecure        bool
	AccessKeySecret ArtifactRepositorySecret `yaml:"accessKeySecret"`
	SecretKeySecret ArtifactRepositorySecret `yaml:"secretKeySecret"`
	AccessKey       string                   `yaml:"accessKey,omitempty"`
	Secretkey       string                   `yaml:"secretKey,omitempty"`
	Container       string                   `yaml:"container"`
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
