package stock

import (
	"context"
	"crypto/tls"
	"io"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/freeconf/gconf/c2"
	"github.com/freeconf/gconf/meta"
	"github.com/freeconf/gconf/node"
	"github.com/freeconf/gconf/nodes"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type HttpServerOptions struct {
	Addr                     string
	Port                     string
	ReadTimeout              int
	WriteTimeout             int
	Tls                      *Tls
	Iface                    string
	CallbackAddress          string
	NotifyKeepaliveTimeoutMs int
}

type HttpServer struct {
	options HttpServerOptions
	server  *http.Server
	handler http.Handler
}

func (service *HttpServer) Options() HttpServerOptions {
	return service.options
}

func (service *HttpServer) ApplyOptions(options HttpServerOptions) {
	if options == service.options {
		return
	}
	service.options = options
	chkStartErr := func(err error) {
		if err != nil && err != http.ErrServerClosed {
			c2.Err.Fatal(err)
		}
	}
	service.server = &http.Server{
		Addr:           options.Port,
		Handler:        service.handler,
		ReadTimeout:    time.Duration(options.ReadTimeout) * time.Millisecond,
		WriteTimeout:   time.Duration(options.WriteTimeout) * time.Millisecond,
		MaxHeaderBytes: 1 << 20,
	}
	if options.Tls != nil {
		service.server.TLSConfig = &options.Tls.Config
		conn, err := net.Listen("tcp", options.Port)
		if err != nil {
			panic(err)
		}
		tlsListener := tls.NewListener(conn, &options.Tls.Config)
		go func() {
			chkStartErr(service.server.Serve(tlsListener))
		}()
	} else {
		// HTTP/2 w/o TLS is non-standard but possible. Use cases usually
		// involve proxying traffic thru a local (loopback or unix socket)
		// connection. Using http over https often simplifies deployment
		// and increases performance however this should not be used to
		// avoid security to clients outside this server.
		//
		//  https://github.com/golang/go/issues/14141
		h2cServer := &http2.Server{}
		service.server.Handler = h2c.NewHandler(service.handler, h2cServer)
		go func() {
			chkStartErr(service.server.ListenAndServe())
		}()
	}
}

func (service *HttpServer) Stop() {
	// TODO : currently hangs
	service.server.Shutdown(context.Background())
}

func NewHttpServer(handler http.Handler) *HttpServer {
	return &HttpServer{
		handler: handler,
	}
}

func (service *HttpServer) GetHttpClient() *http.Client {
	var client *http.Client
	if service.options.Tls != nil {
		tlsConfig := &tls.Config{
			Certificates: service.options.Tls.Config.Certificates,
			RootCAs:      service.options.Tls.Config.RootCAs,
		}
		transport := &http.Transport{TLSClientConfig: tlsConfig}
		client = &http.Client{Transport: transport}
	} else {
		client = http.DefaultClient
	}
	return client
}

type StreamSourceWebHandler struct {
	Source meta.StreamSource
}

func (service StreamSourceWebHandler) ServeHTTP(wtr http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "" {
		path = "index.html"
	}
	if rdr, err := service.Source.OpenStream(path, ""); err != nil {
		http.Error(wtr, err.Error(), 404)
	} else {
		if closer, ok := rdr.(io.Closer); ok {
			defer closer.Close()
		}
		ext := filepath.Ext(path)
		ctype := mime.TypeByExtension(ext)
		wtr.Header().Set("Content-Type", ctype)
		if _, err = io.Copy(wtr, rdr); err != nil {
			http.Error(wtr, err.Error(), http.StatusInternalServerError)
		}
		// Eventually support this but need file seeker to do that.
		// http.ServeContent(wtr, req, path, time.Now(), &ReaderPeeker{rdr})
	}
}

func WebServerNode(service *HttpServer) node.Node {
	options := service.Options()
	return &nodes.Extend{
		Base: nodes.ReflectChild(&options),
		OnChild: func(p node.Node, r node.ChildRequest) (node.Node, error) {
			switch r.Meta.Ident() {
			case "tls":
				if r.New {
					options.Tls = &Tls{}
				}
				if options.Tls != nil {
					return TlsNode(options.Tls), nil
				}
			}
			return nil, nil
		},
		OnEndEdit: func(p node.Node, r node.NodeRequest) error {
			if err := p.EndEdit(r); err != nil {
				return err
			}
			service.ApplyOptions(options)
			return nil
		},
	}
}
