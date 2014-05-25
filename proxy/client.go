package proxy

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/getlantern/flashlight/log"
	"github.com/getlantern/flashlight/protocol"
	"github.com/getlantern/go-mitm/mitm"
	"github.com/getlantern/go-reverseproxy/rp"
)

type Client struct {
	ProxyConfig
	UpstreamHost        string
	Protocol            protocol.ClientProtocol // host-spoofing protocol to use (e.g. CloudFlare)
	ShouldProxyLoopback bool                    // if true, even requests to the loopback interface are sent to the server proxy
	mitmHandler         http.Handler
}

func (client *Client) Run() error {
	err := client.CertContext.InitCommonCerts()
	if err != nil {
		return fmt.Errorf("Unable to init common certs: %s", err)
	}

	// Note - in practice, this only applies when running on Linux.
	client.InstallCACertToTrustStoreIfNecessary()

	client.buildReverseProxy()

	err = client.buildMITMHandler()
	if err != nil {
		return fmt.Errorf("Unable to build MITM handler: %s", err)
	}

	httpServer := &http.Server{
		Addr:         client.Addr,
		ReadTimeout:  client.ReadTimeout,
		WriteTimeout: client.WriteTimeout,
		Handler:      client.mitmHandler,
	}

	log.Debugf("About to start client (http) proxy at %s", client.Addr)
	return httpServer.ListenAndServe()
}

// buildReverseProxy builds the httputil.ReverseProxy used by the client to
// proxy requests upstream.
func (client *Client) buildReverseProxy() {
	client.reverseProxy = &rp.ReverseProxy{
		Director: func(req *http.Request) {
			// Check for local addresses, which we don't rewrite
			if client.ShouldProxyLoopback || isNotLoopback(req.Host) {
				client.Protocol.RewriteRequest(req)
			}
			if client.ShouldDumpHeaders {
				dumpHeaders("Request", req.Header)
			}
		},
		Transport: withRewrite(
			client.Protocol.RewriteResponse,
			client.ShouldDumpHeaders,
			&http.Transport{
				Dial: func(network, addr string) (net.Conn, error) {
					return client.Protocol.Dial(addr)
				},
			}),
		DynamicFlushInterval: flushIntervalFor,
	}
}

// buildMITMHandler builds the MITM handler that the client uses for proxying
// HTTPS requests. We have to MITM these because we can't CONNECT tunnel through
// CloudFlare.
func (client *Client) buildMITMHandler() (err error) {
	cryptoConf := &mitm.CryptoConfig{
		PKFile:          client.CertContext.PKFile,
		CertFile:        client.CertContext.CACertFile,
		ServerTLSConfig: client.TLSConfig,
	}
	if cryptoConf.ServerTLSConfig == nil {
		cryptoConf.ServerTLSConfig = DEFAULT_TLS_SERVER_CONFIG
	}
	client.mitmHandler, err = mitm.Wrap(client.reverseProxy, cryptoConf)
	if err != nil {
		return fmt.Errorf("Unable to initialize mitm handler: %s", err)
	}
	return nil
}

func (config *ProxyConfig) InstallCACertToTrustStoreIfNecessary() {
	err := config.CertContext.InstallCACertToTrustStoreIfNecessary()
	if err != nil {
		log.Debugf("Unable to install CA Cert to trust store, man in the middling may not work.  Suggest running flashlight as sudo with the -install flag: %s", err)
	}
}

// InstallCACertToTrustStoreIfNecessary installs the CA certificate to the
// system trust store if it hasn't already been installed.  This usually
// requires flashlight to be running with root/Administrator privileges.
func (ctx *CertContext) InstallCACertToTrustStoreIfNecessary() error {
	haveInstalledCert, err := ctx.caCert.IsInstalled()
	if err != nil {
		return fmt.Errorf("Unable to check if CA certificate is installed: %s", err)
	}
	if !haveInstalledCert {
		log.Debugf("Adding CA cert to trust store as trusted root")
		// TODO: add the cert as trusted root anytime that it's not already
		// in the system keystore
		if err = ctx.caCert.AddAsTrustedRoot(); err != nil {
			return err
		}
	} else {
		log.Debugf("CA cert already found in trust store, not adding")
	}
	return nil
}

func isNotLoopback(addr string) bool {
	ip, err := net.ResolveIPAddr("ip4", strings.Split(addr, ":")[0])
	return err == nil && !ip.IP.IsLoopback()
}
