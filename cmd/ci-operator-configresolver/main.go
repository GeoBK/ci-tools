package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/openshift/ci-tools/pkg/api"

	"github.com/sirupsen/logrus"
	prowConfig "k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/flagutil"
	"k8s.io/test-infra/prow/interrupts"
	"k8s.io/test-infra/prow/metrics"
	"k8s.io/test-infra/prow/pjutil"

	"github.com/openshift/ci-tools/pkg/httphelper"
	"github.com/openshift/ci-tools/pkg/load/agents"
	"github.com/openshift/ci-tools/pkg/webreg"
)

type options struct {
	configPath   string
	registryPath string
	prowPath     string
	jobPath      string
	logLevel     string
	address      string
	uiAddress    string
	gracePeriod  time.Duration
	cycle        time.Duration
	validateOnly bool
	flatRegistry bool
}

var (
	configresolverMetrics = httphelper.NewMetrics("configresolver")
)

func gatherOptions() (options, error) {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&o.configPath, "config", "", "Path to config dirs")
	fs.StringVar(&o.registryPath, "registry", "", "Path to registry dirs")
	fs.StringVar(&o.prowPath, "prow-config", "", "Path to prow config")
	fs.StringVar(&o.jobPath, "jobs", "", "Path to job config dir")
	fs.StringVar(&o.logLevel, "log-level", "info", "Level at which to log output.")
	fs.StringVar(&o.address, "address", ":8080", "Address to run server on")
	fs.StringVar(&o.uiAddress, "ui-address", ":8082", "Address to run the registry UI on")
	fs.DurationVar(&o.gracePeriod, "gracePeriod", time.Second*10, "Grace period for server shutdown")
	fs.DurationVar(&o.cycle, "cycle", time.Minute*2, "Cycle duration for config reload")
	fs.BoolVar(&o.validateOnly, "validate-only", false, "Load the config and registry, validate them and exit.")
	fs.BoolVar(&o.flatRegistry, "flat-registry", false, "Disable directory structure based registry validation")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return o, fmt.Errorf("failed to parse flags: %w", err)
	}
	return o, nil
}

func validateOptions(o options) error {
	_, err := logrus.ParseLevel(o.logLevel)
	if err != nil {
		return fmt.Errorf("invalid --log-level: %w", err)
	}
	if o.cycle == 0 {
		return fmt.Errorf("invalid cycle: duration cannot equal 0")
	}
	if o.configPath == "" {
		return fmt.Errorf("--config is required")
	}
	if _, err := os.Stat(o.configPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("--config points to a nonexistent directory: %w", err)
		}
		return fmt.Errorf("Error getting stat info for --config directory: %w", err)
	}
	if o.registryPath == "" {
		return fmt.Errorf("--registry is required")
	}
	if _, err := os.Stat(o.registryPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("--registry points to a nonexistent directory: %w", err)
		}
		return fmt.Errorf("Error getting stat info for --registry directory: %w", err)
	}
	if o.prowPath == "" {
		return fmt.Errorf("--prow-config is required")
	}
	if _, err := os.Stat(o.prowPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("--prow-config points to a nonexistent file: %w", err)
		}
		return fmt.Errorf("Error getting stat info for --prow-config file: %w", err)
	}
	if o.validateOnly && o.flatRegistry {
		return errors.New("--validate-only and --flat-registry flags cannot be set simultaneously")
	}
	return nil
}

func resolveConfig(configAgent agents.ConfigAgent, registryAgent agents.RegistryAgent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(http.StatusText(http.StatusNotImplemented)))
			return
		}
		metadata, err := webreg.MetadataFromQuery(w, r)
		if err != nil {
			configresolverMetrics.RecordError("invalid query")
		}
		logger := logrus.WithFields(api.LogFieldsFor(metadata))

		config, err := configAgent.GetMatchingConfig(metadata)
		if err != nil {
			configresolverMetrics.RecordError("config not found")
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "failed to get config: %v", err)
			logger.WithError(err).Warning("failed to get config")
			return
		}
		resolveAndRespond(registryAgent, config, w, logger)
	}
}

func resolveLiteralConfig(registryAgent agents.RegistryAgent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(http.StatusText(http.StatusNotImplemented)))
			return
		}

		encoded, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Could not read unresolved config from request body."))
			return
		}
		unresolvedConfig := api.ReleaseBuildConfiguration{}
		if err = json.Unmarshal(encoded, &unresolvedConfig); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Could not parse request body as unresolved config."))
			return
		}
		resolveAndRespond(registryAgent, unresolvedConfig, w, &logrus.Entry{})
	}
}

func resolveAndRespond(registryAgent agents.RegistryAgent, config api.ReleaseBuildConfiguration, w http.ResponseWriter, logger *logrus.Entry) {
	config, err := registryAgent.ResolveConfig(config)
	if err != nil {
		configresolverMetrics.RecordError("failed to resolve config with registry")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to resolve config with registry: %v", err)
		logger.WithError(err).Warning("failed to resolve config with registry")
		return
	}
	jsonConfig, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		configresolverMetrics.RecordError("failed to marshal config")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to marshal config to JSON: %v", err)
		logger.WithError(err).Errorf("failed to marshal config to JSON")
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(jsonConfig); err != nil {
		logrus.WithError(err).Error("Failed to write response")
	}
}

func getConfigGeneration(agent agents.ConfigAgent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "%d", agent.GetGeneration())
	}
}

func getRegistryGeneration(agent agents.RegistryAgent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "%d", agent.GetGeneration())
	}
}

func main() {
	o, err := gatherOptions()
	if err != nil {
		logrus.WithError(err).Fatal("failed go gather options")
	}
	if err := validateOptions(o); err != nil {
		logrus.Fatalf("invalid options: %v", err)
	}
	level, _ := logrus.ParseLevel(o.logLevel)
	logrus.SetLevel(level)
	health := pjutil.NewHealth()
	metrics.ExposeMetrics("ci-operator-configresolver", prowConfig.PushGateway{}, flagutil.DefaultMetricsPort)

	configAgent, err := agents.NewConfigAgent(o.configPath, o.cycle, configresolverMetrics.ErrorRate)
	if err != nil {
		logrus.Fatalf("Failed to get config agent: %v", err)
	}

	registryAgent, err := agents.NewRegistryAgent(o.registryPath, o.cycle, configresolverMetrics.ErrorRate, o.flatRegistry)
	if err != nil {
		logrus.Fatalf("Failed to get registry agent: %v", err)
	}

	if o.validateOnly {
		os.Exit(0)
	}

	// add handler func for incorrect paths as well; can help with identifying errors/404s caused by incorrect paths
	http.HandleFunc("/", configresolverMetrics.HandleWithMetrics(http.NotFound))
	http.HandleFunc("/config", configresolverMetrics.HandleWithMetrics(resolveConfig(configAgent, registryAgent)))
	http.HandleFunc("/resolve", configresolverMetrics.HandleWithMetrics(resolveLiteralConfig(registryAgent)))
	http.HandleFunc("/configGeneration", configresolverMetrics.HandleWithMetrics(getConfigGeneration(configAgent)))
	http.HandleFunc("/registryGeneration", configresolverMetrics.HandleWithMetrics(getRegistryGeneration(registryAgent)))
	interrupts.ListenAndServe(&http.Server{Addr: o.address}, o.gracePeriod)
	uiServer := &http.Server{
		Addr:    o.uiAddress,
		Handler: configresolverMetrics.HandleWithMetrics(webreg.WebRegHandler(registryAgent, configAgent)),
	}
	interrupts.ListenAndServe(uiServer, o.gracePeriod)
	health.ServeReady()
	interrupts.WaitForGracefulShutdown()
}
