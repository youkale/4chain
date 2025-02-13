package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/echogy-io/echogy/pkg/auth"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/echogy-io/echogy"
	"github.com/echogy-io/echogy/pkg/logger"
	"github.com/echogy-io/echogy/pkg/pprof"
	"github.com/rs/zerolog"
)

var _conf = flag.String("c", "config.json", "config file, format json")
var _pidFile = flag.String("pid", "", "pid file path (default: executable directory)")

func logLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	}
	return zerolog.WarnLevel
}

func setOSEnv() {
	os.Setenv("COLORTERM", "truecolor")
	os.Setenv("TERM", "xterm-256color")
	os.Setenv("CLICOLOR", "1")
	os.Setenv("CLICOLOR_FORCE", "1")
	os.Setenv("FORCE_COLOR", "1")
	os.Setenv("TERM_PROGRAM", "xterm")
}

type AuthConfig struct {
	PubKeys   []*auth.PubKeyAuth   `json:"pubKeys"`
	Passwords []*auth.PasswordAuth `json:"passwords"`
}

type SysConfig struct {
	LogLevel    string `json:"logLevel"`
	LogFile     string `json:"logFile"` // Path to log file
	EnablePProf bool   `json:"pprof"`
	*echogy.Config
	Auth *AuthConfig `json:"auth"`
}

func main() {

	flag.Parse()

	// Create PID file
	pidPath := *_pidFile
	if pidPath == "" {
		execPath, err := os.Executable()
		if err != nil {
			panic(err)
		}
		pidPath = execPath + ".pid"
	}

	setOSEnv()

	// Write PID to file
	pid := os.Getpid()
	if err := os.WriteFile(pidPath, []byte(fmt.Sprint(pid)), 0644); err != nil {
		panic(err)
	}
	defer os.Remove(pidPath)

	f, err := os.ReadFile(*_conf)

	if nil != err {
		panic(err)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	var sysConfig SysConfig

	err = json.Unmarshal(f, &sysConfig)

	if nil != err {
		panic(fmt.Sprintf("Failed to setup log file: %v", err))
	}

	logger.SetLogLevel(logLevel(sysConfig.LogLevel))

	// Setup log file output if configured
	if sysConfig.LogFile != "" {
		if err := logger.AddFileOutput(sysConfig.LogFile); err != nil {
			panic(fmt.Sprintf("Failed to setup log file: %v", err))
		}
		logger.Info("Log file output enabled", logger.Fields{"path": sysConfig.LogFile})
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)

	if sysConfig.EnablePProf {
		go func() {
			pprof.Serve()
		}()
	}

	_auth := auth.New(sysConfig.Auth.PubKeys, sysConfig.Auth.Passwords)

	go func() {
		echogy.Serve(ctx, sysConfig.Config, _auth)
	}()
	<-c
	logger.WarnN("Echogy will be shutdown")
	cancelFunc()
	time.Sleep(1 * time.Second)
}
