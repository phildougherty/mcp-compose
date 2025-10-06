package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

type Registry struct {
	url        string
	logger     *logging.Logger
	httpClient *http.Client
}

type RegistryImage struct {
	Name        string   `json:"name"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
}

type RegistryConfig struct {
	URL     string
	Timeout time.Duration
}

func NewRegistry(config RegistryConfig, logger *logging.Logger) *Registry {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Registry{
		url:    strings.TrimSuffix(config.URL, "/"),
		logger: logger,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

func (r *Registry) HealthCheck() error {
	url := fmt.Sprintf("%s/v2/", r.url)
	resp, err := r.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("registry health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registry health check returned status %d: %s", resp.StatusCode, string(body))
	}

	r.logger.Info("Docker registry at %s is healthy", r.url)

	return nil
}

func (r *Registry) ListImages() ([]RegistryImage, error) {
	url := fmt.Sprintf("%s/v2/_catalog", r.url)
	resp, err := r.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	var catalog struct {
		Repositories []string `json:"repositories"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return nil, fmt.Errorf("failed to decode catalog: %w", err)
	}

	var images []RegistryImage
	for _, repo := range catalog.Repositories {
		tags, err := r.getImageTags(repo)
		if err != nil {
			r.logger.Warning("Failed to get tags for %s: %v", repo, err)

			continue
		}

		images = append(images, RegistryImage{
			Name: repo,
			Tags: tags,
		})
	}

	return images, nil
}

func (r *Registry) getImageTags(imageName string) ([]string, error) {
	url := fmt.Sprintf("%s/v2/%s/tags/list", r.url, imageName)
	resp, err := r.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	var tagsList struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tagsList); err != nil {
		return nil, fmt.Errorf("failed to decode tags: %w", err)
	}

	return tagsList.Tags, nil
}

func (r *Registry) GetImageManifest(imageName, tag string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v2/%s/manifests/%s", r.url, imageName, tag)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	var manifest map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	return manifest, nil
}

func (r *Registry) PullImage(imageName, tag string) error {
	manifest, err := r.GetImageManifest(imageName, tag)
	if err != nil {
		return fmt.Errorf("failed to verify image: %w", err)
	}

	r.logger.Info("Image %s:%s verified in registry", imageName, tag)
	r.logger.Debug("Manifest: %v", manifest)

	return nil
}

func (r *Registry) ImageExists(imageName, tag string) (bool, error) {
	_, err := r.GetImageManifest(imageName, tag)
	if err != nil {
		if strings.Contains(err.Error(), "status 404") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (r *Registry) GetURL() string {
	return r.url
}
