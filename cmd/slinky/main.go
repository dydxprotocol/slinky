package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	//nolint: gosec
	_ "net/http/pprof"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/dydxprotocol/slinky/cmd/build"
	cmdconfig "github.com/dydxprotocol/slinky/cmd/slinky/config"
	"github.com/dydxprotocol/slinky/oracle"
	"github.com/dydxprotocol/slinky/oracle/config"
	oraclemetrics "github.com/dydxprotocol/slinky/oracle/metrics"
	"github.com/dydxprotocol/slinky/pkg/log"
	oraclemath "github.com/dydxprotocol/slinky/pkg/math/oracle"
	"github.com/dydxprotocol/slinky/providers/apis/marketmap"
	oraclefactory "github.com/dydxprotocol/slinky/providers/factories/oracle"
	mmservicetypes "github.com/dydxprotocol/slinky/service/clients/marketmap/types"
	oracleserver "github.com/dydxprotocol/slinky/service/servers/oracle"
	promserver "github.com/dydxprotocol/slinky/service/servers/prometheus"
	mmtypes "github.com/dydxprotocol/slinky/x/marketmap/types"
)

var (
	rootCmd = &cobra.Command{
		Use:   "oracle",
		Short: "Run the slinky oracle server.",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runOracle()
		},
	}

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version of the oracle.",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(build.Build)
		},
	}

	// oracle config flags.
	flagMetricsEnabled           = "metrics-enabled"
	flagMetricsPrometheusAddress = "metrics-prometheus-address"
	flagHost                     = "host"
	flagPort                     = "port"
	flagUpdateInterval           = "update-interval"
	flagMaxPriceAge              = "max-price-age"

	// flag-bound values.
	oracleCfgPath       string
	marketCfgPath       string
	marketMapProvider   string
	updateMarketCfgPath string
	runPprof            bool
	profilePort         string
	logLevel            string
	fileLogLevel        string
	writeLogsTo         string
	marketMapEndPoint   string
	maxLogSize          int
	maxBackups          int
	maxAge              int
	disableCompressLogs bool
	disableRotatingLogs bool
)

const (
	DefaultLegacyConfigPath = "./oracle.json"
)

func init() {
	rootCmd.Flags().StringVarP(
		&marketMapProvider,
		"marketmap-provider",
		"",
		marketmap.Name,
		"MarketMap provider to use (marketmap_api, dydx_api, dydx_migration_api).",
	)
	rootCmd.Flags().StringVarP(
		&oracleCfgPath,
		"oracle-config",
		"",
		"",
		"Path to the oracle config file.",
	)
	rootCmd.Flags().StringVarP(
		&marketCfgPath,
		"market-config-path",
		"",
		"",
		"Path to the market config file. If you supplied a node URL in your config, this will not be required.",
	)
	rootCmd.Flags().StringVarP(
		&updateMarketCfgPath,
		"update-market-config-path",
		"",
		"",
		"Path where the current market config will be written. Overwrites any pre-existing file. Requires an http-node-url/marketmap provider in your oracle.json config.",
	)
	rootCmd.Flags().BoolVarP(
		&runPprof,
		"run-pprof",
		"",
		false,
		"Run pprof server.",
	)
	rootCmd.Flags().StringVarP(
		&profilePort,
		"pprof-port",
		"",
		"6060",
		"Port for the pprof server to listen on.",
	)
	rootCmd.Flags().StringVarP(
		&logLevel,
		"log-std-out-level",
		"",
		"info",
		"Log level (debug, info, warn, error, dpanic, panic, fatal).",
	)
	rootCmd.Flags().StringVarP(
		&fileLogLevel,
		"log-file-level",
		"",
		"info",
		"Log level for the file logger (debug, info, warn, error, dpanic, panic, fatal).",
	)
	rootCmd.Flags().StringVarP(
		&writeLogsTo,
		"log-file",
		"",
		"sidecar.log",
		"Write logs to a file.",
	)
	rootCmd.Flags().IntVarP(
		&maxLogSize,
		"log-max-size",
		"",
		100,
		"Maximum size in megabytes before log is rotated.",
	)
	rootCmd.Flags().IntVarP(
		&maxBackups,
		"log-max-backups",
		"",
		1,
		"Maximum number of old log files to retain.",
	)
	rootCmd.Flags().IntVarP(
		&maxAge,
		"log-max-age",
		"",
		3,
		"Maximum number of days to retain an old log file.",
	)
	rootCmd.Flags().BoolVarP(
		&disableCompressLogs,
		"log-file-disable-compression",
		"",
		false,
		"Compress rotated log files.",
	)
	rootCmd.Flags().BoolVarP(
		&disableRotatingLogs,
		"log-disable-file-rotation",
		"",
		false,
		"Disable writing logs to a file.",
	)
	rootCmd.Flags().StringVarP(
		&marketMapEndPoint,
		"market-map-endpoint",
		"",
		"",
		"Use a custom listen-to endpoint for market-map (overwrites what is provided in oracle-config).",
	)

	// these flags are connected to the OracleConfig.
	rootCmd.Flags().Bool(
		flagMetricsEnabled,
		cmdconfig.DefaultMetricsEnabled,
		"Enables the Oracle client metrics",
	)
	rootCmd.Flags().String(
		flagMetricsPrometheusAddress,
		cmdconfig.DefaultPrometheusServerAddress,
		"Sets the Prometheus server address for the Oracle client metrics",
	)
	rootCmd.Flags().String(
		flagHost,
		cmdconfig.DefaultHost,
		"The address the Oracle serve from",
	)
	rootCmd.Flags().String(
		flagPort,
		cmdconfig.DefaultPort,
		"The port the Oracle will serve from",
	)
	rootCmd.Flags().Int(
		flagUpdateInterval,
		cmdconfig.DefaultUpdateInterval,
		"The interval at which the oracle will fetch prices from providers",
	)
	rootCmd.Flags().Duration(
		flagMaxPriceAge,
		cmdconfig.DefaultMaxPriceAge,
		"Maximum age of a price that the oracle will consider valid",
	)
	// bind them to viper.
	err := errors.Join(
		viper.BindPFlag("host", rootCmd.Flags().Lookup(flagHost)),
		viper.BindPFlag("port", rootCmd.Flags().Lookup(flagPort)),
		viper.BindPFlag("metrics.enabled", rootCmd.Flags().Lookup(flagMetricsEnabled)),
		viper.BindPFlag("metrics.prometheusServerAddress", rootCmd.Flags().Lookup(flagMetricsPrometheusAddress)),
		viper.BindPFlag("maxPriceAge", rootCmd.Flags().Lookup(flagMaxPriceAge)),
		viper.BindPFlag("updateInterval", rootCmd.Flags().Lookup(flagUpdateInterval)),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to bind flags: %v", err))
	}

	rootCmd.MarkFlagsMutuallyExclusive("update-market-config-path", "market-config-path")
	rootCmd.MarkFlagsMutuallyExclusive("market-map-endpoint", "market-config-path")

	rootCmd.AddCommand(versionCmd)
}

// start the oracle-grpc server + oracle process, cancel on interrupt or terminate.
func main() {
	rootCmd.Execute()
}

func runOracle() error {
	// channel with width for either signal
	sigs := make(chan os.Signal, 1)

	// gracefully trigger close on interrupt or terminate signals
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up logging.
	logCfg := log.NewDefaultConfig()
	logCfg.StdOutLogLevel = logLevel
	logCfg.FileOutLogLevel = fileLogLevel
	logCfg.DisableRotating = disableRotatingLogs
	logCfg.WriteTo = writeLogsTo
	logCfg.MaxSize = maxLogSize
	logCfg.MaxBackups = maxBackups
	logCfg.MaxAge = maxAge
	logCfg.Compress = !disableCompressLogs

	// Build logger.
	logger := log.NewLogger(logCfg)
	defer logger.Sync()

	var cfg config.OracleConfig
	var err error

	cfg, err = cmdconfig.ReadOracleConfigWithOverrides(oracleCfgPath, marketMapProvider)
	if err != nil {
		return fmt.Errorf("failed to get oracle config: %w", err)
	}

	// overwrite endpoint
	if marketMapEndPoint != "" {
		cfg, err = overwriteMarketMapEndpoint(cfg, marketMapEndPoint)
		if err != nil {
			return fmt.Errorf("failed to overwrite market endpoint %s: %w", marketMapEndPoint, err)
		}
	}

	// check that the marketmap endpoint they provided is correct.
	if marketMapProvider == marketmap.Name {
		mmEndpoint := cfg.Providers[marketMapProvider].API.Endpoints[0].URL
		if err := isValidGRPCEndpoint(mmEndpoint); err != nil {
			return err
		}
	}

	var marketCfg mmtypes.MarketMap
	if marketCfgPath != "" {
		marketCfg, err = mmtypes.ReadMarketMapFromFile(marketCfgPath)
		if err != nil {
			return fmt.Errorf("failed to read market config file: %w", err)
		}
	}

	logger.Info(
		"successfully read in configs",
		zap.String("oracle_config_path", oracleCfgPath),
		zap.String("market_config_path", marketCfgPath),
	)

	metrics := oraclemetrics.NewMetricsFromConfig(cfg.Metrics)
	aggregator, err := oraclemath.NewIndexPriceAggregator(
		logger,
		marketCfg,
		metrics,
	)
	if err != nil {
		return fmt.Errorf("failed to create data aggregator: %w", err)
	}

	// Define the oracle options. These determine how the oracle is created & executed.
	oracleOpts := []oracle.Option{
		oracle.WithLogger(logger),
		oracle.WithMarketMap(marketCfg),
		oracle.WithPriceAPIQueryHandlerFactory(oraclefactory.APIQueryHandlerFactory),             // Replace with custom API query handler factory.
		oracle.WithPriceWebSocketQueryHandlerFactory(oraclefactory.WebSocketQueryHandlerFactory), // Replace with custom websocket query handler factory.
		oracle.WithMarketMapperFactory(oraclefactory.MarketMapProviderFactory),
		oracle.WithMetrics(metrics),
	}
	if updateMarketCfgPath != "" {
		oracleOpts = append(oracleOpts, oracle.WithWriteTo(updateMarketCfgPath))
	}

	// Create the oracle and start the oracle.
	orc, err := oracle.New(
		cfg,
		aggregator,
		oracleOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to create oracle: %w", err)
	}
	go func() {
		if err := orc.Start(ctx); err != nil {
			logger.Fatal("failed to start oracle", zap.Error(err))
		}
	}()
	defer orc.Stop()

	srv := oracleserver.NewOracleServer(orc, logger)

	// cancel oracle on interrupt or terminate
	go func() {
		<-sigs
		logger.Info("received interrupt or terminate signal; closing oracle")

		cancel()
	}()

	// start prometheus metrics
	if cfg.Metrics.Enabled {
		logger.Info("starting prometheus metrics", zap.String("address", cfg.Metrics.PrometheusServerAddress))
		ps, err := promserver.NewPrometheusServer(cfg.Metrics.PrometheusServerAddress, logger)
		if err != nil {
			return fmt.Errorf("failed to start prometheus metrics: %w", err)
		}

		go ps.Start()

		// close server on shut-down
		go func() {
			<-ctx.Done()
			logger.Info("stopping prometheus metrics")
			ps.Close()
		}()
	}

	if runPprof {
		endpoint := fmt.Sprintf("%s:%s", cfg.Host, profilePort)
		// Start pprof server
		go func() {
			logger.Info("Starting pprof server", zap.String("endpoint", endpoint))
			if err := http.ListenAndServe(endpoint, nil); err != nil { //nolint: gosec
				logger.Error("pprof server failed", zap.Error(err))
			}
		}()
	}

	// start server (blocks).
	if err := srv.StartServer(ctx, cfg.Host, cfg.Port); err != nil {
		logger.Error("stopping server", zap.Error(err))
	}
	return nil
}

func overwriteMarketMapEndpoint(cfg config.OracleConfig, overwrite string) (config.OracleConfig, error) {
	for providerName, provider := range cfg.Providers {
		if provider.Type == mmservicetypes.ConfigType {
			provider.API.Endpoints = []config.Endpoint{
				{
					URL: overwrite,
				},
			}
			cfg.Providers[providerName] = provider
			return cfg, cfg.ValidateBasic()
		}
	}

	return cfg, fmt.Errorf("no market-map provider found in config")
}

// isValidGRPCEndpoint checks that the string s is a valid gRPC endpoint. (doesn't start with http, ends with a port).
func isValidGRPCEndpoint(s string) error {
	if strings.HasPrefix(s, "http") {
		return fmt.Errorf("expected gRPC endpoint but got HTTP endpoint %q. Please provide a gRPC endpoint (e.g. some.host:9090)", s)
	}
	if !hasPort(s) {
		// they might do something like foo.bar:hello
		// so lets just take the bit before foo.bar for the example in the error.
		example := strings.Split(s, ":")[0]
		return fmt.Errorf("invalid gRPC endpoint %q. Must specify port (e.g. %s:9090)", s, example)
	}
	return nil
}

// hasPort reports whether s contains `:` followed by numbers.
func hasPort(s string) bool {
	// matches anything that has `:` and some numbers after.
	pattern := `:[0-9]+$`

	regex := regexp.MustCompile(pattern)

	return regex.MatchString(s)
}
