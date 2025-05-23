package server

import (
	"context"
	"fmt"
	fhttp "github.com/bogdanfinn/fhttp"
	utls "github.com/bogdanfinn/utls"
	"io"
	"net"
)

// ConfigurationHeaderKey is the name of the header field that contains the RoundTripper configuration.
// Note that this key can only start with one capital letter and the rest in lowercase.
// Unfortunately, this seems to be a limitation of Burp's Extender API.
const ConfigurationHeaderKey = "Awesometlsconfig"

var (
	s         *fhttp.Server
	proxy     *interceptProxy
	isProxyOn bool
)

func init() {
	s = &fhttp.Server{}
}

func StartServer(addr string) error {
	if addr == "" {
		return fmt.Errorf("address must be provided")
	}

	s = &fhttp.Server{}

	ca, private, err := NewCertificateAuthority()
	if err != nil {
		return fmt.Errorf("NewCertificateAuthority, err: %w", err)
	}

	m := fhttp.NewServeMux()
	m.HandleFunc("/", func(w fhttp.ResponseWriter, req *fhttp.Request) {
		configHeader := req.Header.Get(ConfigurationHeaderKey)
		req.Header.Del(ConfigurationHeaderKey)

		config, err := ParseTransportConfig(configHeader)
		if err != nil {
			writeError(w, err)
			return
		}

		if !isProxyOn && config.UseInterceptedFingerprint {
			if err = StartProxy(config.InterceptProxyAddr, config.BurpAddr); err != nil {
				writeError(w, err)
				return
			}
			isProxyOn = true
		} else if isProxyOn && !config.UseInterceptedFingerprint {
			if err = StopProxy(); err != nil {
				writeError(w, err)
				return
			}
			isProxyOn = false
		}

		if proxy != nil {
			if interceptedFingerprint := proxy.getTLSFingerprint(); interceptedFingerprint != "" && config.UseInterceptedFingerprint {
				config.HexClientHello = HexClientHello(interceptedFingerprint)
			}
		}

		client, err := NewClient(config)
		if err != nil {
			writeError(w, err)
			return
		}

		req.URL.Host = config.Host
		req.URL.Scheme = config.Scheme
		req.RequestURI = ""
		req.Header[fhttp.HeaderOrderKey] = config.HeaderOrder
		// The content-length header is already set by the client (internally).
		// Leaving it here causes strange '400 bad request' errors from the destination, so we remove it.
		req.Header.Del("Content-Length")

		res, err := client.Do(req)
		if err != nil {
			writeError(w, err)
			return
		}

		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			writeError(w, err)
			return
		}

		// Write the response (back to burp).
		for k := range res.Header {
			vv := res.Header.Values(k)
			for _, v := range vv {
				// The response body is already automatically decompressed, so we need to update the Content-Length header accordingly.
				// Not doing so will cause the response writer to return an error.
				if k == "Content-Length" {
					w.Header().Add(k, fmt.Sprintf("%d", len(body)))
				} else {
					w.Header().Add(k, v)
				}
			}
		}
		w.WriteHeader(res.StatusCode)
		w.Write(body)
	})

	s.Addr = addr
	s.Handler = m
	s.TLSConfig = &utls.Config{
		Certificates: []utls.Certificate{
			{
				Certificate: [][]byte{ca.Raw},
				PrivateKey:  private,
				Leaf:        ca,
			},
		},
		NextProtos: []string{"http/1.1"},
	}

	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("listen, err: %w", err)
	}

	tlsListener := utls.NewListener(listener, s.TLSConfig)

	if err := s.Serve(tlsListener); err != nil {
		return fmt.Errorf("serve, err: %w", err)
	}

	return nil
}

func StartProxy(interceptAddr, burpAddr string) (err error) {
	p, err := newInterceptProxy(interceptAddr, burpAddr)
	if err != nil {
		return err
	}

	proxy = p

	go proxy.Start()

	return nil
}

func StopProxy() (err error) {
	if proxy == nil {
		return nil
	}
	return proxy.Stop()
}

func StopServer() error {
	return s.Shutdown(context.Background())
}

func writeError(w fhttp.ResponseWriter, err error) {
	w.WriteHeader(500)
	fmt.Fprint(w, fmt.Errorf("Awesome TLS error: %s", err))
	fmt.Println(err)
}
