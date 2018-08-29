package main

//go:generate go run $GOPATH/src/v2ray.com/core/common/errors/errorgen/main.go -pkg main -path Main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"v2ray.com/core"
	"v2ray.com/core/common/platform"
	_ "v2ray.com/core/main/distro/all"
	"v2ray.com/core/app/dispatcher"
	"v2ray.com/core/common/net"
	"v2ray.com/core/app/log"
	clog "v2ray.com/core/common/log"
	"v2ray.com/core/transport/internet"
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/proxy/vmess/inbound"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/serial"
	"v2ray.com/core/proxy/freedom"
	"v2ray.com/core/proxy/vmess"
)

var (
	configFile = flag.String("config", "", "Config file for V2Ray.")
	version    = flag.Bool("version", false, "Show current version of V2Ray.")
	test       = flag.Bool("test", false, "Test config file only, without launching V2Ray server.")
	format     = flag.String("format", "json", "Format of input file.")
	plugin     = flag.Bool("plugin", false, "True to load plugins.")
)

func fileExists(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}

func getConfigFilePath() string {
	if len(*configFile) > 0 {
		return *configFile
	}

	if workingDir, err := os.Getwd(); err == nil {
		configFile := filepath.Join(workingDir, "config.json")
		if fileExists(configFile) {
			return configFile
		}
	}

	if configFile := platform.GetConfigurationPath(); fileExists(configFile) {
		return configFile
	}

	return ""
}

func GetConfigFormat() string {
	switch strings.ToLower(*format) {
	case "pb", "protobuf":
		return "protobuf"
	default:
		return "json"
	}
}

func withDefaultApps(config *core.Config) *core.Config {
	config.App = append(config.App, serial.ToTypedMessage(&dispatcher.Config{}))
	config.App = append(config.App, serial.ToTypedMessage(&proxyman.InboundConfig{}))
	config.App = append(config.App, serial.ToTypedMessage(&proxyman.OutboundConfig{}))
	return config
}

func startV2Ray() (core.Server, error) {
	//configFile := getConfigFilePath()
	//configInput, err := confloader.LoadConfig(configFile)
	//if err != nil {
	//	return nil, newError("failed to load config: ", configFile).Base(err)
	//}
	//defer configInput.Close()

	//config, err := core.LoadConfig(GetConfigFormat(), configFile, configInput)
	//if err != nil {
	//	return nil, newError("failed to read config file: ", configFile).Base(err)
	//}


	fmt.Println("************************************")
	//fmt.Println(config)
        //rawReceiverSettings := config.Inbound[0].ReceiverSettings
	//fmt.Println(rawReceiverSettings)
	//rs = rawReceiverSettings.ToTypedMessage()
        //receiverSettings, _ := rawReceiverSettings.(*proxyman.ReceiverConfig)
        //fmt.Println(receiverSettings.Listen.AsAddress())
        //fmt.Println(receiverSettings.PortRange)
	//host := os.Getenv("HOST")
	port := os.Getenv("PORT")
	host := "0.0.0.0"
	userID := os.Getenv("UUID")
	fmt.Println("host: ", host)
	fmt.Println("port: ", port)
	fmt.Println("UUID: ", userID)
	//host := "0.0.0.0"
	//port := "443"
	serverPort, _ := net.PortFromString(port)
	serverIP := net.ParseAddress(host)
	serverConfig := &core.Config{
		Inbound: []*core.InboundHandlerConfig{
			{
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortRange: net.SinglePortRange(serverPort),
					Listen:    net.NewIPOrDomain(serverIP),
					StreamSettings: &internet.StreamConfig{
						TransportSettings: []*internet.TransportConfig{
							{
								Protocol: internet.TransportProtocol_WebSocket,
							},
						},
					},
				}),
				ProxySettings: serial.ToTypedMessage(&inbound.Config{
					User: []*protocol.User{
						{
							Account: serial.ToTypedMessage(&vmess.Account{
								Id:      userID,
								AlterId: 64,
							}),
						},
					},
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&freedom.Config{}),
			},
		},
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&log.Config{
				ErrorLogLevel: clog.Severity_Debug,
				ErrorLogType:  log.LogType_Console,
			}),
		},
	}
	sc := withDefaultApps(serverConfig)
	//fmt.Println("************************************")
	//fmt.Println(sc)
        //rawReceiverSettings = serverConfig.Inbound[0].ReceiverSettings
	//fmt.Println(rawReceiverSettings)
	server, err := core.New(sc)
	if err != nil {
		return nil, newError("failed to create server").Base(err)
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
	flag.Parse()

	printVersion()

	if *version {
		return
	}

	if *plugin {
		if err := core.LoadPlugins(); err != nil {
			fmt.Println("Failed to load plugins:", err.Error())
			os.Exit(-1)
		}
	}

	server, err := startV2Ray()
	if err != nil {
		fmt.Println(err.Error())
		// Configuration error. Exit with a special value to prevent systemd from restarting.
		os.Exit(23)
	}

	if *test {
		fmt.Println("Configuration OK.")
		os.Exit(0)
	}

	if err := server.Start(); err != nil {
		fmt.Println("Failed to start", err)
		os.Exit(-1)
	}

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)

	<-osSignals
	server.Close()
}
