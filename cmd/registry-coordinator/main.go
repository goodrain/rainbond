//go:build linux || darwin

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net"
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

var errConfiguration = errors.New("invalid registry coordinator configuration")

type tlsFiles struct{ ca, certificate, key, serverName string }

func (files *tlsFiles) flags(flags *flag.FlagSet, prefix string) {
	flags.StringVar(&files.ca, prefix+"-ca-file", "", "trusted CA bundle")
	flags.StringVar(&files.certificate, prefix+"-client-cert-file", "", "client TLS certificate")
	flags.StringVar(&files.key, prefix+"-client-key-file", "", "client TLS key file")
	flags.StringVar(&files.serverName, prefix+"-server-name", "", "verified TLS server name")
}
func (files tlsFiles) transport() (http.RoundTripper, error) {
	config := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: files.serverName}
	if files.ca != "" {
		raw, err := os.ReadFile(files.ca)
		if err != nil {
			return nil, errConfiguration
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(raw) {
			return nil, errConfiguration
		}
		config.RootCAs = roots
	}
	if (files.certificate == "") != (files.key == "") {
		return nil, errConfiguration
	}
	if files.certificate != "" {
		certificate, err := tls.LoadX509KeyPair(files.certificate, files.key)
		if err != nil {
			return nil, errConfiguration
		}
		config.Certificates = []tls.Certificate{certificate}
	}
	return &http.Transport{TLSClientConfig: config, DialContext: (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 90 * time.Second, ExpectContinueTimeout: time.Second}, nil
}
func readCredential(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", errConfiguration
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(raw) > 8192 {
		return "", errConfiguration
	}
	value := strings.TrimSpace(string(raw))
	if len(value) < 32 || strings.ContainsAny(value, " \t\r\n") {
		return "", errConfiguration
	}
	return value, nil
}
func run(ctx context.Context, args []string) error {
	return runWithOutput(ctx, args, os.Stdout)
}

func runWithOutput(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("registry-coordinator", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var binding coordination.StorageRegistration
	var listen, root, upstream, api, credential, owner, serverCert, serverKey, pod, podUID string
	var initialize, measure, allowHTTP bool
	var controlTLS, upstreamTLS tlsFiles
	flags.StringVar(&listen, "listen", ":5001", "proxy listen address")
	flags.StringVar(&root, "storage-root", "", "mounted Registry data root")
	flags.StringVar(&binding.StorageID, "storage-id", "", "control-plane storage identity")
	flags.StringVar(&binding.Generation, "storage-generation", "", "control-plane storage generation")
	flags.StringVar(&binding.VolumeUID, "volume-uid", "", "verified physical volume identity")
	flags.StringVar(&binding.RootPath, "registry-path", "", "data root inside the original Registry container")
	flags.StringVar(&upstream, "upstream", "http://127.0.0.1:5000", "loopback Registry origin")
	flags.StringVar(&api, "coordination-api", "", "trusted Region API origin")
	flags.StringVar(&credential, "credential-file", "", "mounted Region coordination credential")
	flags.StringVar(&owner, "owner", "", "instance identity prefix supplied by the installer")
	flags.StringVar(&pod, "pod-name", os.Getenv("POD_NAME"), "Pod name from the downward API")
	flags.StringVar(&podUID, "pod-uid", os.Getenv("POD_UID"), "Pod UID from the downward API")
	flags.StringVar(&serverCert, "tls-cert-file", "", "public listener TLS certificate")
	flags.StringVar(&serverKey, "tls-key-file", "", "public listener TLS key")
	flags.BoolVar(&initialize, "initialize-storage-identity", false, "installer-only atomic identity initialization")
	flags.BoolVar(&measure, "measure-storage", false, "print verified backing filesystem capacity without starting the proxy")
	flags.BoolVar(&allowHTTP, "allow-internal-http", false, "allow a trusted private HTTP Region API")
	controlTLS.flags(flags, "coordination")
	upstreamTLS.flags(flags, "upstream")
	if flags.Parse(args) != nil || flags.NArg() != 0 || root == "" {
		return errConfiguration
	}
	if _, err := binding.Fingerprint(); err != nil {
		return errConfiguration
	}
	if initialize && measure {
		return errConfiguration
	}
	if measure {
		if err := ctx.Err(); err != nil {
			return err
		}
		observed, err := registryproxy.MeasureStorage(root, binding)
		if err != nil {
			return err
		}
		if output == nil {
			return errConfiguration
		}
		return json.NewEncoder(output).Encode(observed)
	}
	if initialize {
		return registryproxy.InitializeStorageIdentity(root, binding)
	}
	if owner == "" || len(owner) > 95 || strings.ContainsAny(owner, "\r\n\x00") || pod == "" || podUID == "" || (serverCert == "") != (serverKey == "") {
		return errConfiguration
	}
	nonce, err := coordination.NewActivationRevision()
	if err != nil {
		return errConfiguration
	}
	owner += ":" + nonce
	token, err := readCredential(credential)
	if err != nil {
		return err
	}
	controlTransport, err := controlTLS.transport()
	if err != nil {
		return err
	}
	upstreamTransport, err := upstreamTLS.transport()
	if err != nil {
		return err
	}
	client, err := coordination.NewCoordinationClient(api, token, allowHTTP, controlTransport)
	if err != nil {
		return errConfiguration
	}
	runtime, err := registryproxy.NewRuntime(registryproxy.RuntimeConfig{Root: root, Binding: binding, Upstream: upstream, Owner: owner, Pod: pod, PodUID: podUID, Backend: client, Transport: upstreamTransport, PermitKey: func() []byte {
		value, err := readCredential(credential)
		if err != nil {
			return nil
		}
		return []byte(value)
	}})
	if err != nil {
		return err
	}
	server := &http.Server{Handler: runtime, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 64 << 10}
	if serverCert != "" {
		certificate, err := tls.LoadX509KeyPair(serverCert, serverKey)
		if err != nil {
			return errConfiguration
		}
		server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}
	}
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return errors.New("registry coordinator listener unavailable")
	}
	defer listener.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if server.Shutdown(shutdown) != nil {
				server.Close()
			}
		case <-done:
		}
	}()
	if serverCert != "" {
		err = server.ServeTLS(listener, "", "")
	} else {
		err = server.Serve(listener)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		logrus.Fatal("Registry coordinator startup or serving failed; inspect configuration and readiness")
	}
}
