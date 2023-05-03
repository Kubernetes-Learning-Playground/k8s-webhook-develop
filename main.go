package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"k8s.io/klog"
	"k8sWebhookPractice/pkg"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {

	var parameters pkg.TLSServerParameters

	// get command line parameters
	// 都会使用默认值
	flag.StringVar(&parameters.CertFile, "tlsCertFile", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.KeyFile, "tlsKeyFile", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()

	klog.Info(fmt.Sprintf("port=%d, cert-file=%s, key-file=%s", parameters.Port, parameters.CertFile, parameters.KeyFile))

	file, err := tls.LoadX509KeyPair(parameters.CertFile, parameters.KeyFile)
	if err != nil {
		klog.Errorf("Fail to load tls file %s", err)
	}

	tlsServer := &pkg.TLSServer{
		Server: &http.Server{
			Addr: fmt.Sprintf(":%v", os.Getenv("PORT")),
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{file},
			},
		},
		WhiteOrBlock:        os.Getenv("WhITE_OR_BLOCK"),
		WhiteListRegistries: strings.Split(os.Getenv("WHITELIST_REGISTRIES"), ","),
		BlackListRegistries: strings.Split(os.Getenv("BLACKLIST_REGISTRIES"), ","),
		MutateObject:        os.Getenv("MUTATE_OBJECT"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/validate", tlsServer.Serve)
	mux.HandleFunc("/mutate", tlsServer.Serve)

	tlsServer.Server.Handler = mux

	// 启动https server
	go func() {
		if err := tlsServer.Server.ListenAndServeTLS("", ""); err != nil {
			klog.Errorf("Fail to listen and serve webhook server: %v", err)
		}
	}()

	klog.Info("Server start!!")

	// 优雅关闭
	stopC := make(chan os.Signal, 1)
	signal.Notify(stopC, syscall.SIGINT, syscall.SIGTERM)
	<-stopC
	klog.Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
	if err := tlsServer.Server.Shutdown(context.Background()); err != nil {
		klog.Errorf("HTTP server Shutdown: %v", err)
	}

}
