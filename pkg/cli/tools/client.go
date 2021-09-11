package tools

import (
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

func (c *ToolsClient) List() (*CLIToolList, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/tools/", c.endpoint), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	list := &CLIToolList{}
	if err := json.NewDecoder(resp.Body).Decode(list); err != nil {
		return nil, err
	}

	return list, nil
}

func (c *ToolsClient) Download(tool CLITool, operatingSystem, architecture, destination string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/tools/download/?namespace=%s&name=%s&os=%s&arch=%s", c.endpoint, tool.Namespace, tool.Name, operatingSystem, architecture), nil)
	if err != nil {
		return err
	}

	resp, err := c.rt.RoundTrip(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		obj := &struct {
			Error string `json:"error"`
		}{}

		if err := json.NewDecoder(resp.Body).Decode(obj); err != nil {
			return err
		}

		return fmt.Errorf(obj.Error)
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
