/*
 *     Copyright 2020 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package proxy

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"github.com/pkg/errors"

	logger "d7y.io/dragonfly/v2/internal/dflog"
)

func (proxy *Proxy) ServeSNI(l net.Listener) error {
	if proxy.cert == nil {
		return errors.New("empty cert")
	}
	if proxy.cert.Leaf != nil && proxy.cert.Leaf.IsCA {
		logger.Infof("hijack sni https request with CA <%s>", proxy.cert.Leaf.Subject.CommonName)
	} else {
		logger.Warnf("cert is not ca cert, may be cause tls cert error")
	}

	var port = portHTTPS
	if tcpAddr, ok := l.Addr().(*net.TCPAddr); ok {
		port = tcpAddr.Port
	}
	// TODO Stop
	for {
		conn, err := l.Accept()
		if err != nil {
			logger.Errorf("accept connection error: %s", err)
			continue
		}
		go proxy.handleTLSConn(conn, port)
	}
}

// handshakeTLSConn performs the TLS handshake.
func handshakeTLSConn(clientConn net.Conn, config *tls.Config) (net.Conn, error) {
	conn := tls.Server(clientConn, config)
	if err := conn.Handshake(); err != nil {
		conn.Close()
		clientConn.Close()
		return nil, err
	}
	return conn, nil
}

func (proxy *Proxy) handleTLSConn(clientConn net.Conn, port int) {
	var serverName string
	sConfig := new(tls.Config)
	if !proxy.cert.Leaf.IsCA {
		sConfig.Certificates = []tls.Certificate{*proxy.cert}
	} else {
		if proxy.certCache == nil { // Initialize proxy.certCache on first access. (Lazy init)
			proxy.certCache = lru.New(100) // Default max entries size = 100
		}
		leafCertSpec := LeafCertSpec{
			proxy.cert.Leaf.PublicKey,
			proxy.cert.PrivateKey,
			proxy.cert.Leaf.SignatureAlgorithm}
		sConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// It's assumed that `hello.ServerName` is always same as `host`, in practice.
			serverName = hello.ServerName
			cached, hit := proxy.certCache.Get(serverName)
			if hit && time.Now().Before(cached.(*tls.Certificate).Leaf.NotAfter) { // If cache hit and the cert is not expired
				logger.Debugf("TLS Cache hit, cacheKey = <%s>", serverName)
				return cached.(*tls.Certificate), nil
			}
			logger.Debugf("Generate temporal leaf TLS cert for ServerName <%s>", hello.ServerName)
			cert, err := genLeafCert(proxy.cert, &leafCertSpec, serverName)
			if err == nil {
				// Put cert in cache only if there is no error. So all certs in cache are always valid.
				// But certs in cache maybe expired (After 24 hours, see the default duration of generated certs)
				proxy.certCache.Add(serverName, cert)
			}
			// If err != nil, means unrecoverable error happened in genLeafCert(...)
			return cert, err
		}
	}

	tlsConn, err := handshakeTLSConn(clientConn, sConfig)
	if err != nil {
		logger.Errorf("handshake failed for %s: %v", serverName, err)
		return
	}
	defer tlsConn.Close()

	rp := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = schemaHTTPS
			r.URL.Host = serverName
			if port != portHTTPS {
				r.URL.Host = fmt.Sprintf("%s:%d", serverName, port)
			}
		},
		Transport: proxy.newTransport(proxy.remoteConfig(serverName)),
	}

	// We have to wait until the connection is closed
	wg := sync.WaitGroup{}
	wg.Add(1)
	// NOTE: http.Serve always returns a non-nil error
	if err := http.Serve(&singleUseListener{&customCloseConn{tlsConn, wg.Done}}, rp); err != errServerClosed && err != http.ErrServerClosed {
		logger.Errorf("failed to accept incoming HTTPS connections: %v", err)
	}
	wg.Wait()
}
