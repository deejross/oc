package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	apiSubdomain      = "api."
	appsSubdomain     = "apps."
	controllerRoute   = "api-openshift-cli-manager."
	controllerPingKey = "name"
	controllerPingVal = "openshift-cli-manager"
)

var (
	toolsDescription = templates.LongDesc(`
		Manage CLI tools on this machine.
`)
	toolsExample = templates.Examples(`
		# List installed tools
		oc tools

		# Get list of available tools
		oc tools --available

		# Install a tool on this machine
		oc tools --install kubectl

		# Remove a tool from this machine
		oc tools --remove kubectl
`)
)

type ToolsOptions struct {
	Available  bool
	Install    string
	Remove     string
	BinaryPath string
	Address    string
	client     *ToolsClient

	genericclioptions.IOStreams
}

func NewToolsOptions(streams genericclioptions.IOStreams) *ToolsOptions {
	return &ToolsOptions{
		IOStreams: streams,
	}
}

func NewCmdTools(f kcmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewToolsOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "tools",
		Short:   "Manage CLI tools on this machine",
		Long:    toolsDescription,
		Example: toolsExample,
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, c, args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&o.Available, "available", false, "List available tools")
	flags.StringVar(&o.Install, "install", "", "Install a tool on this machine")
	flags.StringVar(&o.Remove, "remove", "", "Remove a tool from this machine")
	flags.StringVar(&o.BinaryPath, "binary-path", "", "Path for binaries (default's to user's `bin` directory")
	flags.StringVar(&o.Address, "address", "", "The address for the openshift-cli-manager service (auto-discovered)")
	return cmd
}

func (o *ToolsOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(o.BinaryPath) == 0 {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		o.BinaryPath = filepath.Join(homeDir, "bin")
		_, err = os.Stat(o.BinaryPath)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(o.BinaryPath, 0755); err != nil {
				return err
			}
		}
	}

	_, err := os.Stat(o.BinaryPath)
	if err != nil {
		return err
	}

	o.client, err = NewToolsClient(f, o.Address)
	return err
}

func (o *ToolsOptions) Run() error {
	if len(o.Remove) > 0 {
		return o.remove()
	} else if len(o.Install) > 0 {
		return o.install()
	} else if o.Available {
		return o.available()
	}
	return o.installed()
}

func (o *ToolsOptions) available() error {
	list, err := o.client.List()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(o.Out, 0, 4, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "\tNAME\tDESCRIPTION\n")

	for _, tool := range list.Items {
		for _, bin := range tool.Spec.Binaries {
			if bin.Architecture == runtime.GOARCH && bin.OS == runtime.GOOS {
				fmt.Fprintf(w, "\t%s\t%s\n", tool.Name, tool.Spec.Description)
				break
			}
		}
	}

	return nil
}

func (o *ToolsOptions) installed() error {
	list, err := o.client.List()
	if err != nil {
		return err
	}

	tools := map[string]CLITool{}
	for _, tool := range list.Items {
		tools[tool.Name] = tool
	}

	files, err := os.ReadDir(o.BinaryPath)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(o.Out, 0, 4, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "\tNAME\tDESCRIPTION\n")

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := strings.TrimSuffix(filepath.Base(file.Name()), ".exe")
		if tool, ok := tools[name]; ok {
			fmt.Fprintf(w, "\t%s\t%s\n", tool.Name, tool.Spec.Description)
		}
	}

	return nil
}

func (o *ToolsOptions) install() error {
	name := o.Install

	list, err := o.client.List()
	if err != nil {
		return err
	}

	for _, tool := range list.Items {
		if tool.Name == name {
			path := filepath.Join(o.BinaryPath, tool.Name)
			if runtime.GOOS == "windows" {
				path += ".exe"
			}

			return o.client.Download(tool, runtime.GOOS, runtime.GOARCH, path)
		}
	}

	return fmt.Errorf("tool %s not found", name)
}

func (o *ToolsOptions) remove() error {
	path := filepath.Join(o.BinaryPath, o.Remove)
	if runtime.GOOS == "windows" {
		path += ".exe"
	}

	return os.Remove(path)
}
