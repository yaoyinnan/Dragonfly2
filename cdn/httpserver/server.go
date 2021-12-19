package httpserver

import (
	"context"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	"d7y.io/dragonfly/v2/cdn/httpserver/proxy"
	logger "d7y.io/dragonfly/v2/internal/dflog"
)

type Server struct {
	config     Config
	proxy      *proxy.Proxy
	httpServer *http.Server
}

func New(config Config, rpcServer *grpc.Server) (*Server, error) {
	grpc_prometheus.Register(rpcServer)
	proxy, err := proxy.NewProxy()
	if err != nil {
		return nil, err
	}
	httpServer := &http.Server{}
	server := &Server{
		config:     config,
		httpServer: httpServer,
		proxy:      proxy,
	}
	httpServer.Handler = createRouter(server)
	return server, nil
}

// ListenAndServe is a blocking call which runs s.
func (s *Server) ListenAndServe() error {
	l, err := net.Listen(s.config.Net, s.config.Addr)
	if err != nil {
		return err
	}
	logger.Infof("====starting http server at %s====", s.config.Addr)
	err = s.httpServer.Serve(l)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func createRouter(s *Server) *mux.Router {
	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler())
	r.Handle("/proxy", s.proxy)
	return r
}

func (s *Server) Shutdown(ctx context.Context) error {
	defer logger.Infof("====stopped http server====")
	return s.httpServer.Shutdown(ctx)
}
