package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"quan-agent/internal/config"
	"quan-agent/internal/deepseek"
	"quan-agent/internal/inventory"
	"quan-agent/internal/ops"
	"quan-agent/internal/remote"
	"quan-agent/internal/server"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:18090", "本地监听地址")
	noBrowser := flag.Bool("no-browser", false, "启动后不自动打开浏览器")
	flag.Parse()
	if err := validateListenAddress(*listen); err != nil {
		fmt.Fprintln(os.Stderr, "监听地址无效:", err)
		os.Exit(2)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	store, err := config.NewStore()
	if err != nil {
		logger.Error("初始化配置失败", "error", err)
		os.Exit(1)
	}

	toolbox := ops.NewToolbox()
	inventoryStore, err := inventory.NewStore()
	if err != nil {
		logger.Error("初始化机器清单失败", "error", err)
		os.Exit(1)
	}
	runner := remote.NewRunner(inventoryStore)
	chatClient := deepseek.NewClient(store, toolbox)
	handler := server.New(store, inventoryStore, chatClient, toolbox, runner, logger)
	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("Quan Agent 已启动", "url", "http://"+*listen)
		errCh <- httpServer.ListenAndServe()
	}()

	if !*noBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			if err := openBrowser("http://" + *listen); err != nil {
				logger.Warn("无法自动打开浏览器", "error", err)
			}
		}()
	}

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-stopCh:
		logger.Info("收到退出信号", "signal", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("服务异常退出", "error", err)
			os.Exit(1)
		}
	}

	if err := httpServer.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "关闭服务失败:", err)
	}
}

func openBrowser(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func validateListenAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("为保护密钥和机器信息，只允许监听 127.0.0.1、::1 或 localhost")
	}
	return nil
}
