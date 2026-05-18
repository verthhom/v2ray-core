package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	syscall "syscall"

	"v2ray.com/core"
	"v2ray.com/core/common/cmdarg"
	"v2ray.com/core/common/platform"
	_ "v2ray.com/core/main/distro/all"
)

var (
	configFiles cmdarg.Arg // -config option, support multiple config files
	configDir   string
	version     = flag.Bool("version", false, "Show current version of V2Ray.")
	testConfig  = flag.Bool("test", false, "Test config file only, without launching V2Ray server.")
	format      = flag.String("format", "json", "Format of input file.")
)

func fileExists(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}

func dirExists(file string) bool {
	info, err := os.Stat(file)
	return err == nil && info.IsDir()
}

func getConfigFilePath() cmdarg.Arg {
	if dirExists(configDir) {
		conftDirAbsPath, err := filepath.Abs(configDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to get absolute path of config directory: %v\n", err)
			return nil
		}
		return cmdarg.Arg{conftDirAbsPath}
	}

	if len(configFiles) > 0 {
		return configFiles
	}

	if workingDir, err := os.Getwd(); err == nil {
		defaultPath := filepath.Join(workingDir, "config.json")
		if fileExists(defaultPath) {
			return cmdarg.Arg{defaultPath}
		}
	}

	if configFile := platform.GetConfigurationPath(); fileExists(configFile) {
		return cmdarg.Arg{configFile}
	}

	return nil
}

func startV2Ray() (core.Server, error) {
	configFiles := getConfigFilePath()

	if len(configFiles) == 0 {
		return nil, fmt.Errorf("no config file found; use -config to specify one")
	}

	config, err := core.LoadConfig(strings.ToLower(*format), configFiles[0], configFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to read config files: %s", err.Error())
	}

	server, err := core.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %s", err.Error())
	}

	return server, nil
}

func printVersion() {
	version := core.VersionStatement()
	for _, s := range version {
		fmt.Println(s)
	}
}

func main() {
	// Register -config flag with support for multiple values
	flag.Var(&configFiles, "config", "Config file for V2Ray. Multiple assignment is accepted (e.g., -config file1.json -config file2.json).")
	flag.Var(&configFiles, "c", "Short alias of -config.")
	flag.StringVar(&configDir, "confdir", "", "A directory with multiple config files. They will be merged as a single config.")

	flag.Parse()

	// Only print version info when explicitly requested, not on every startup.
	// This keeps the log output cleaner when running as a systemd service.
	if *version {
		printVersion()
		return
	}

	server, err := startV2Ray()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		// Exit with a specific code so that systemd can handle it correctly
		os.Exit(23)
	}

	if *testConfig {
		fmt.Println("Configuration OK.")
		return
	}

	if err := server.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to start:", err)
		os.Exit(1)
	}
	defer server.Close()

	// Print the OS and architecture on startup so it's easy to confirm the
	// running environment when checking logs (useful for cross-compiled builds).
	fmt.Printf("V2Ray started on %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Wait for interrupt or termination signal before shutting down.
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
	<-osSignals
}
