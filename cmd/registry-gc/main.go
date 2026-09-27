//go:build linux || darwin

// registry-gc is a one-shot, Job-bound executor, never a scheduler or server.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/registryproxy"
	"github.com/sirupsen/logrus"
)

var errConfiguration = errors.New("invalid GC executor configuration")

type gcInvocation func(context.Context, string, coordination.StorageRegistration, coordination.CoordinationRequest, string, registryproxy.GCExecutionRecorder, bool) error

func runGC(ctx context.Context, args []string, invoke gcInvocation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet("registry-gc", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var configFile, credentialFile, endpoint, caFile, certFile, keyFile, serverName string
	var allowHTTP, recover bool
	flags.StringVar(&configFile, "configuration-file", "", "server-generated operation descriptor")
	flags.StringVar(&credentialFile, "credential-file", "", "mounted internal coordination credential")
	flags.StringVar(&endpoint, "coordination-api", "", "trusted Region API origin")
	flags.StringVar(&serverName, "coordination-server-name", "", "verified API TLS server name")
	flags.StringVar(&caFile, "coordination-ca-file", "", "trusted API CA bundle")
	flags.StringVar(&certFile, "coordination-client-cert-file", "", "mounted API client certificate")
	flags.StringVar(&keyFile, "coordination-client-key-file", "", "mounted API client key")
	flags.BoolVar(&allowHTTP, "allow-internal-http", false, "allow explicitly trusted private HTTP")
	flags.BoolVar(&recover, "recover", false, "report original durable receipt without executing GC")
	if flags.Parse(args) != nil || flags.NArg() != 0 || invoke == nil {
		return errConfiguration
	}
	var descriptor struct {
		Binding coordination.StorageRegistration `json:"binding"`
		Request coordination.CoordinationRequest `json:"request"`
	}
	input := os.Getenv("CLEANUP_GC_OPERATION")
	var source io.Reader
	if configFile != "" {
		if input != "" {
			return errConfiguration
		}
		file, err := os.Open(configFile)
		if err != nil {
			return errConfiguration
		}
		defer file.Close()
		source = io.LimitReader(file, 16385)
	} else {
		if input == "" || len(input) > 16384 {
			return errConfiguration
		}
		source = strings.NewReader(input)
	}
	decoder := json.NewDecoder(source)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&descriptor) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errConfiguration
	}
	// StorageID is intentionally omitted from CoordinationRequest JSON. Resolve
	// it only from the operation's persisted storage binding.
	descriptor.Request.StorageID = descriptor.Binding.StorageID
	locator := coordination.GCExecutorLocator{Pod: os.Getenv("POD_NAME"), PodUID: os.Getenv("POD_UID")}
	if !locator.Valid() {
		return errConfiguration
	}
	secret, err := os.Open(credentialFile)
	if err != nil {
		return errConfiguration
	}
	defer secret.Close()
	raw, err := io.ReadAll(io.LimitReader(secret, 8193))
	if err != nil || len(raw) > 8192 {
		return errConfiguration
	}
	token := strings.TrimSpace(string(raw))
	if token == "" || strings.ContainsAny(token, " \r\n\t") {
		return errConfiguration
	}
	configuration := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}
	if caFile != "" {
		ca, err := os.ReadFile(caFile)
		if err != nil {
			return errConfiguration
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			return errConfiguration
		}
		configuration.RootCAs = pool
	}
	if (certFile == "") != (keyFile == "") {
		return errConfiguration
	}
	if certFile != "" {
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return errConfiguration
		}
		configuration.Certificates = []tls.Certificate{certificate}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = configuration
	transport.ResponseHeaderTimeout = 10 * time.Second
	defer transport.CloseIdleConnections()
	client, err := coordination.NewCoordinationClient(endpoint, token, allowHTTP, transport)
	if err != nil {
		return errConfiguration
	}
	recorder, err := registryproxy.NewGCJobRecorder(client, descriptor.Binding, descriptor.Request, locator)
	if err != nil {
		return errConfiguration
	}
	// The helper and native executable come from the verified immutable Job image.
	// No caller-controlled executable path or native GC flags are accepted.
	return invoke(ctx, descriptor.Binding.RootPath, descriptor.Binding, descriptor.Request, "/bin/registry", recorder, recover)
}

func execute(ctx context.Context, root string, binding coordination.StorageRegistration, request coordination.CoordinationRequest, binary string, recorder registryproxy.GCExecutionRecorder, recover bool) error {
	if recover {
		return registryproxy.RecoverGCReceipt(ctx, root, binding, request, recorder)
	}
	return registryproxy.ExecuteGC(ctx, root, binding, request, binary, recorder)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	if err := runGC(ctx, os.Args[1:], execute); err != nil {
		// Do not expose configuration, credentials, native output or upstream bodies.
		logrus.Error("GC executor did not complete; inspect the recorded operation before taking further action")
		os.Exit(1)
	}
}
