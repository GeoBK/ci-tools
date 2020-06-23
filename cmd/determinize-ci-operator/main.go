package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/openshift/ci-tools/pkg/api"
	"github.com/openshift/ci-tools/pkg/config"
)

func gatherOptions() config.ConfirmableOptions {
	o := config.ConfirmableOptions{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	o.Bind(fs)
	if err := fs.Parse(os.Args[1:]); err != nil {
		logrus.WithError(err).Fatal("could not parse input")
	}
	return o
}

func main() {
	o := gatherOptions()
	if err := o.Validate(); err != nil {
		logrus.Fatalf("Invalid options: %v", err)
	}

	var toCommit []config.DataWithInfo
	if err := o.OperateOnCIOperatorConfigDir(o.ConfigDir, func(configuration *api.ReleaseBuildConfiguration, info *config.Info) error {
		output := config.DataWithInfo{Configuration: *configuration, Info: *info}
		if !o.Confirm {
			output.Logger().Info("Would re-format file.")
			return nil
		}

		// we treat the filepath as the ultimate source of truth for this
		// data, but we record it in the configuration files to ensure that
		// it's easy to consume it for downstream tools
		output.Configuration.Metadata = info.Metadata

		// we are walking the config so we need to commit once we're done
		toCommit = append(toCommit, output)

		return nil
	}); err != nil {
		logrus.WithError(err).Fatal("Could not branch configurations.")
	}

	for _, output := range toCommit {
		if err := output.CommitTo(o.ConfigDir); err != nil {
			logrus.WithError(err).Fatal("commitTo failed")
		}
	}
}
