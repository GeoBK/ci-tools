package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/openshift/ci-tools/pkg/bitwarden"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/test-infra/prow/logrusutil"
)

type options struct {
	logLevel       string
	configPath     string
	bwUser         string
	dryRun         bool
	bwPasswordPath string
	maxConcurrency int

	config     []bitWardenItem
	bwPassword string
}

type bitWardenItem struct {
	ItemName   string         `json:"item_name"`
	Field      fieldGenerator `json:"field,omitempty"`
	Attachment fieldGenerator `json:"attachment,omitempty"`
	Attribute  fieldGenerator `json:"attribute,omitempty"`
}

type fieldGenerator struct {
	Name string `json:"name,omitempty"`
	Cmd  string `json:"cmd,omitempty"`
}

const (
	attributeTypePassword string = "password"
)

func parseOptions() options {
	var o options
	flag.CommandLine.BoolVar(&o.dryRun, "dry-run", false, "Whether to actually create the secrets with oc command")
	flag.CommandLine.StringVar(&o.configPath, "config", "", "Path to the config file to use for this tool.")
	flag.CommandLine.StringVar(&o.bwUser, "bw-user", "", "Username to access BitWarden.")
	flag.CommandLine.StringVar(&o.bwPasswordPath, "bw-password-path", "", "Path to a password file to access BitWarden.")
	flag.CommandLine.StringVar(&o.logLevel, "log-level", "info", fmt.Sprintf("Log level is one of %v.", logrus.AllLevels))
	flag.CommandLine.IntVar(&o.maxConcurrency, "concurrency", 1, "Maximum number of concurrent in-flight goroutines to BitWarden.")
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		logrus.WithError(err).Errorf("cannot parse args: %q", os.Args[1:])
	}
	return o
}

func (o *options) validateOptions() error {
	level, err := logrus.ParseLevel(o.logLevel)
	if err != nil {
		return fmt.Errorf("invalid log level specified: %w", err)
	}
	logrus.SetLevel(level)
	if o.bwUser == "" {
		return fmt.Errorf("--bw-user is empty")
	}
	if o.bwPasswordPath == "" {
		return fmt.Errorf("--bw-password-path is empty")
	}
	if o.configPath == "" {
		return fmt.Errorf("--config is empty")
	}
	return nil
}

func (o *options) completeOptions(secrets sets.String) error {
	bytes, err := ioutil.ReadFile(o.bwPasswordPath)
	if err != nil {
		return err
	}
	o.bwPassword = strings.TrimSpace(string(bytes))
	secrets.Insert(o.bwPassword)

	bytes, err = ioutil.ReadFile(o.configPath)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(bytes, &o.config)
	if err != nil {
		return err
	}
	return o.validateCompletedOptions()
}

func (o *options) validateCompletedOptions() error {
	if o.bwPassword == "" {
		return fmt.Errorf("--bw-password-file was empty")
	}

	for i, bwItem := range o.config {
		if bwItem.ItemName == "" {
			return fmt.Errorf("config[%d].itemName: empty key is not allowed", i)
		}
		if bwItem.Attribute.Name != attributeTypePassword && bwItem.Attribute.Name != "" {
			return fmt.Errorf("config[%d].attribute: only the '%s' is supported, not %s", i, attributeTypePassword, bwItem.Attribute.Name)
		}
		if (bwItem.Field.Name != "" && bwItem.Field.Cmd == "") ||
			(bwItem.Attribute.Name != "" && bwItem.Attribute.Cmd == "") ||
			(bwItem.Attachment.Name != "" && bwItem.Attachment.Cmd == "") {
			return fmt.Errorf("config[%d]: empty field not allowed for cmd if name is specified for any of attribute, field or attachment", i)
		}
	}
	return nil
}

func executeCommand(command string) ([]byte, error) {
	cmd := strings.Fields(command)
	out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("bw cmd '%s' failed, output- %s : %w", command, string(out), err)
	}
	return out, nil
}

func updateSecrets(bwItems []bitWardenItem, bwClient bitwarden.Client) error {
	for _, bwItem := range bwItems {
		if bwItem.Field.Name != "" {
			out, err := executeCommand(bwItem.Field.Cmd)
			if err != nil {
				return fmt.Errorf("failed to set field item: %s, field: %s - %w", bwItem.ItemName, bwItem.Field.Name, err)
			}
			if err := bwClient.SetFieldOnItem(bwItem.ItemName, bwItem.Field.Name, out); err != nil {
				return fmt.Errorf("failed to set field item: %s, field: %s - %w", bwItem.ItemName, bwItem.Field.Name, err)
			}
		}
		if bwItem.Attachment.Name != "" {
			out, err := executeCommand(bwItem.Field.Cmd)
			if err != nil {
				return fmt.Errorf("failed to set attachment, item: %s, attachment: %s - %w", bwItem.ItemName, bwItem.Attachment.Name, err)
			}
			if err := bwClient.SetAttachmentOnItem(bwItem.ItemName, bwItem.Attachment.Name, out); err != nil {
				return fmt.Errorf("failed to set attachment, item: %s, attachment: %s - %w", bwItem.ItemName, bwItem.Attachment.Name, err)
			}
		}
		if bwItem.Attribute.Name != "" {
			out, err := executeCommand(bwItem.Field.Cmd)
			if err != nil {
				return fmt.Errorf("failed to set password, item: %s - %w", bwItem.ItemName, err)
			}
			if err := bwClient.SetPassword(bwItem.ItemName, out); err != nil {
				return fmt.Errorf("failed to set password, item: %s - %w", bwItem.ItemName, err)
			}
		}
	}
	return nil
}

func main() {
	// CLI tool which does the secret generation and uploading to bitwarden
	o := parseOptions()
	secrets := sets.NewString()
	logrus.SetFormatter(logrusutil.NewCensoringFormatter(logrus.StandardLogger().Formatter, func() sets.String {
		return secrets
	}))
	if err := o.validateOptions(); err != nil {
		logrus.WithError(err).Fatal("invalid arguments.")
	}
	if err := o.completeOptions(secrets); err != nil {
		logrus.WithError(err).Fatal("failed to complete options.")
	}
	var client bitwarden.Client
	if o.dryRun {
		tmpFile, err := ioutil.TempFile("", "ci-secret-generator")
		if err != nil {
			logrus.WithError(err).Fatal("failed to create tempfile")
		}
		client, err = bitwarden.NewDryRunClient(tmpFile)
		if err != nil {
			logrus.WithError(err).Fatal("failed to create dryRun client")
		}
		logrus.Infof("Dry-Run enabled, writing secrets to %s", tmpFile.Name())
	} else {
		var err error
		client, err = bitwarden.NewClient(o.bwUser, o.bwPassword, func(s string) {
			secrets.Insert(s)
		})
		if err != nil {
			logrus.WithError(err).Fatal("failed to get Bitwarden client.")
		}
	}
	logrus.RegisterExitHandler(func() {
		if _, err := client.Logout(); err != nil {
			logrus.WithError(err).Fatal("failed to logout.")
		}
	})
	defer logrus.Exit(0)

	// Upload the output to bitwarden
	if err := updateSecrets(o.config, client); err != nil {
		logrus.WithError(err).Fatalf("Failed to update secrets.")
	}
	logrus.Info("Updated secrets.")
}
