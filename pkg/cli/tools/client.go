package tools

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	kcmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type ToolsClient struct {
	f        kcmdutil.Factory
	endpoint string
	config   *rest.Config
	rt       http.RoundTripper
}

func NewToolsClient(f kcmdutil.Factory, address string) (*ToolsClient, error) {
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	c := &ToolsClient{
		f:      f,
		config: config,
	}

	tc, err := config.TransportConfig()
	if err != nil {
		return nil, err
	}

	c.rt, err = transport.New(tc)
	if err != nil {
		return nil, err
	}

	if len(address) == 0 {
		if err := c.detectAddress(); err != nil {
			return nil, err
		}
	} else {
		c.endpoint = address
	}

	return c, nil
}

// ListOptions contains the optional options for the List operation.
type ListOptions struct {
	// Platform as a non-empty string will return only the tools that support the given platform in format `os/arch`.
	Platform string
}

// List available tools.
func (c *ToolsClient) List(opts *ListOptions) (*HTTPCLIToolList, error) {
	if opts == nil {
		opts = &ListOptions{}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/tools/?platform="+url.QueryEscape(opts.Platform), c.endpoint), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.handleResponseError(resp); err != nil {
		return nil, err
	}

	list := &HTTPCLIToolList{}
	if err := json.NewDecoder(resp.Body).Decode(list); err != nil {
		return nil, err
	}

	return list, nil
}

// InfoOptions contains the optional options for the Info operation.
type InfoOptions struct {
	// Version as a non-empty string will return a specific version of the tool, or setting it to `latest` will return the latest version of the tool. Leaving this empty will return all known versions of the tool.
	Version string
}

// Info gets information about a tool.
func (c *ToolsClient) Info(namespace, name, opts *InfoOptions) (*HTTPCLIToolInfo, error) {
	if opts == nil {
		opts = &InfoOptions{}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/tools/info/?namespace=%s&name=%s&version=%s", c.endpoint, namespace, name, opts.Version), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.handleResponseError(resp); err != nil {
		return nil, err
	}

	info := &HTTPCLIToolInfo{}
	if err := json.NewDecoder(resp.Body).Decode(info); err != nil {
		return nil, err
	}

	return info, nil
}

// InfoFromDigest gets information about a tool and its version from the given digest.
func (c *ToolsClient) InfoFromDigest(digest string) (*HTTPCLIToolInfo, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/tools/info/?digest=%s", c.endpoint, digest), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.handleResponseError(resp); err != nil {
		return nil, err
	}

	info := &HTTPCLIToolInfo{}
	if err := json.NewDecoder(resp.Body).Decode(info); err != nil {
		return nil, err
	}

	return info, nil
}

// DownloadOptions contains the optional options for the Download operation.
type DownloadOptions struct {
	// Version as a non-empty string will return a specific version of the tool. Leaving this empty will return the latest version of the tool.
	Version string
}

// Download a tool.
func (c *ToolsClient) Download(namespace, name, platform, destination string, opts *DownloadOptions) error {
	if opts == nil {
		opts = &DownloadOptions{}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/tools/download/?namespace=%s&name=%s&platform=%s&version=%s",
		c.endpoint,
		url.QueryEscape(namespace),
		url.QueryEscape(name),
		url.QueryEscape(platform),
		url.QueryEscape(opts.Version),
	), nil)
	if err != nil {
		return err
	}

	resp, err := c.rt.RoundTrip(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := c.handleResponseError(resp); err != nil {
		return err
	}

	if resp.ContentLength == 0 {
		return fmt.Errorf("binary was not found or could not be extracted")
	}

	f, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
	if err != nil {
		return fmt.Errorf("could not open destination file for writing: %v", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func (c *ToolsClient) detectAddress() error {
	detectionFailedErr := fmt.Errorf("unable to auto-detect openshift-cli-manager address, please set `--address` manually")

	apiURL, err := url.ParseRequestURI(c.config.Host)
	if err != nil {
		return fmt.Errorf("%v: %v", detectionFailedErr, err)
	}

	apiURL.Host = controllerRoute + appsSubdomain + strings.TrimPrefix(apiURL.Hostname(), apiSubdomain)
	c.endpoint = apiURL.String()

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/", c.endpoint), nil)
	if err != nil {
		return fmt.Errorf("%v: %v", detectionFailedErr, err)
	}

	resp, err := c.rt.RoundTrip(req)
	if err != nil {
		return fmt.Errorf("%v: %v", detectionFailedErr, err)
	}
	defer resp.Body.Close()

	m := map[string]string{}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return fmt.Errorf("%v: %v", detectionFailedErr, err)
	}

	if m[controllerPingKey] == controllerPingVal {
		return nil
	}

	return detectionFailedErr
}

func (c *ToolsClient) handleResponseError(resp *http.Response) error {
	if resp.StatusCode >= 400 {
		obj := &struct {
			Error string `json:"error"`
		}{}

		if err := json.NewDecoder(resp.Body).Decode(obj); err != nil {
			return err
		}

		return fmt.Errorf(obj.Error)
	}

	return nil
}

// CalculateDigest calculates the digest from the given filename.
func CalculateDigest(filename string) (string, error) {
	hash := sha256.New()
	f, err := os.OpenFile(filename, os.O_RDONLY, 0755)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}
