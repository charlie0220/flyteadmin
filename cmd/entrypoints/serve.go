package entrypoints

import (
	"context"
	"crypto/tls"
	"fmt"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpcauth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/lyft/flyteadmin/pkg/auth"
	"github.com/lyft/flyteadmin/pkg/server"
	"github.com/pkg/errors"
	"google.golang.org/grpc/credentials"
	"net"
	"net/http"
	_ "net/http/pprof" // Required to serve application.
	"strings"

	"github.com/lyft/flyteadmin/pkg/common"

	"github.com/lyft/flytestdlib/logger"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	flyteService "github.com/lyft/flyteidl/gen/pb-go/flyteidl/service"

	"github.com/lyft/flyteadmin/pkg/config"
	"github.com/lyft/flyteadmin/pkg/rpc/adminservice"

	"github.com/spf13/cobra"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/lyft/flytestdlib/contextutils"
	"github.com/lyft/flytestdlib/promutils/labeled"
	"google.golang.org/grpc"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Launches the Flyte admin server",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		serverConfig := config.GetConfig()

		if serverConfig.Security.Secure {
			return serveGatewaySecure(ctx, serverConfig)
		}
		return serveGatewayInsecure(ctx, serverConfig)
	},
}

func init() {
	// Command information
	RootCmd.AddCommand(serveCmd)

	// Set Keys
	labeled.SetMetricKeys(contextutils.AppNameKey, contextutils.ProjectKey, contextutils.DomainKey,
		contextutils.ExecIDKey, contextutils.WorkflowIDKey, contextutils.NodeIDKey, contextutils.TaskIDKey,
		contextutils.TaskTypeKey, common.RuntimeTypeKey, common.RuntimeVersionKey)
}

// Creates a new gRPC Server with all the configuration
func newGRPCServer(ctx context.Context, cfg *config.ServerConfig, authContext auth.AuthenticationContext,
	opts ...grpc.ServerOption) (*grpc.Server, error) {
	// Not yet implemented for streaming
	var chainedUnaryInterceptors grpc.UnaryServerInterceptor
	if cfg.Security.UseAuth {
		logger.Infof(ctx, "Creating gRPC server with authentication")
		chainedUnaryInterceptors = grpc_middleware.ChainUnaryServer(grpc_prometheus.UnaryServerInterceptor,
			grpcauth.UnaryServerInterceptor(auth.GetAuthenticationInterceptor(authContext)),
			auth.AuthenticationLoggingInterceptor,
		)
	} else {
		logger.Infof(ctx, "Creating gRPC server without authentication")
		chainedUnaryInterceptors = grpc_middleware.ChainUnaryServer(grpc_prometheus.UnaryServerInterceptor)
	}
	serverOpts := []grpc.ServerOption{
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(chainedUnaryInterceptors),
	}
	serverOpts = append(serverOpts, opts...)
	grpcServer := grpc.NewServer(serverOpts...)
	grpc_prometheus.Register(grpcServer)
	flyteService.RegisterAdminServiceServer(grpcServer, adminservice.NewAdminServer(cfg.KubeConfig, cfg.Master))
	return grpcServer, nil
}

// TODO: To be removed, landing page for easier testing
func loginPage(w http.ResponseWriter, _ *http.Request) {
	var htmlIndex = `<html>
<body>
	<a href="/login">Okta Log In</a>
</body>
</html>`
	_, _ = fmt.Fprintf(w, htmlIndex)
}

func GetHandleOpenapiSpec(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		swaggerBytes, err := flyteService.Asset("admin.swagger.json")
		if err != nil {
			logger.Warningf(ctx, "Err %v", err)
			w.WriteHeader(http.StatusFailedDependency)
		} else {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(swaggerBytes)
			if err != nil {
				logger.Errorf(ctx, "failed to write openAPI information, error: %s", err.Error())
			}
		}
	}
}

func newHTTPServer(ctx context.Context, cfg *config.ServerConfig, authContext auth.AuthenticationContext,
	grpcAddress string, grpcConnectionOpts ...grpc.DialOption) (*http.ServeMux, error) {

	// Register the server that will serve HTTP/REST Traffic
	mux := http.NewServeMux()

	// Register healthcheck
	mux.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Register OpenAPI endpoint
	// This endpoint will serve the OpenAPI2 spec generated by the swagger protoc plugin, and bundled by go-bindata
	mux.HandleFunc("/api/v1/openapi", GetHandleOpenapiSpec(ctx))

	// Register the actual Server that will service gRPC traffic
	var gwmux *runtime.ServeMux
	if cfg.Security.UseAuth {
		// Add HTTP handlers for OAuth2 endpoints
		mux.HandleFunc("/login_page", loginPage)
		mux.HandleFunc("/login", auth.RefreshTokensIfExists(ctx, authContext,
			auth.GetLoginHandler(ctx, authContext)))
		mux.HandleFunc("/callback", auth.GetCallbackHandler(ctx, authContext))

		gwmux = runtime.NewServeMux(
			runtime.WithMarshalerOption("application/octet-stream", &runtime.ProtoMarshaller{}),
			runtime.WithMetadata(auth.GetHttpRequestCookieToMetadataHandler(authContext)))
	} else {
		gwmux = runtime.NewServeMux(
			runtime.WithMarshalerOption("application/octet-stream", &runtime.ProtoMarshaller{}))
	}

	err := flyteService.RegisterAdminServiceHandlerFromEndpoint(ctx, gwmux, grpcAddress, grpcConnectionOpts)
	if err != nil {
		return nil, errors.Wrap(err, "error registering admin service")
	}

	mux.Handle("/", gwmux)

	return mux, nil
}

func serveGatewayInsecure(ctx context.Context, cfg *config.ServerConfig) error {
	logger.Infof(ctx, "Serving Flyte Admin Insecure")
	grpcServer, err := newGRPCServer(ctx, cfg, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create GRPC server")
	}

	logger.Infof(ctx, "Serving GRPC Traffic on: %s", cfg.GetGrpcHostAddress())
	lis, err := net.Listen("tcp", cfg.GetGrpcHostAddress())
	if err != nil {
		return errors.Wrapf(err, "failed to listen on GRPC port: %s", cfg.GetGrpcHostAddress())
	}

	go func() {
		err := grpcServer.Serve(lis)
		logger.Fatalf(ctx, "Failed to create GRPC Server, Err: ", err)
	}()

	logger.Infof(ctx, "Starting HTTP/1 Gateway server on %s", cfg.GetHostAddress())
	httpServer, err := newHTTPServer(ctx, cfg, nil, cfg.GetGrpcHostAddress(), grpc.WithInsecure())
	if err != nil {
		return err
	}
	err = http.ListenAndServe(cfg.GetHostAddress(), httpServer)
	if err != nil {
		return errors.Wrapf(err, "failed to Start HTTP Server")
	}

	return nil
}

// grpcHandlerFunc returns an http.Handler that delegates to grpcServer on incoming gRPC
// connections or otherHandler otherwise.
// See https://github.com/philips/grpc-gateway-example/blob/master/cmd/serve.go for reference
func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This is a partial recreation of gRPC's internal checks
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	})
}

func serveGatewaySecure(ctx context.Context, cfg *config.ServerConfig) error {
	certPool, cert, err := server.GetSslCredentials(ctx, cfg.Security.Ssl.CertificateFile, cfg.Security.Ssl.KeyFile)
	if err != nil {
		return err
	}
	// This will parse configuration and create the necessary objects for dealing with auth
	authContext, err := auth.NewAuthenticationContext(ctx, cfg.Security.Oauth)
	if err != nil {
		logger.Errorf(ctx, "Error creating auth context %s", err)
		return err
	}

	grpcServer, err := newGRPCServer(ctx, cfg, authContext,
		grpc.Creds(credentials.NewServerTLSFromCert(cert)))
	if err != nil {
		return errors.Wrap(err, "failed to create GRPC server")
	}

	// Whatever certificate is used, pass it along for easier development
	dialCreds := credentials.NewTLS(&tls.Config{
		ServerName: cfg.GetHostAddress(),
		RootCAs:    certPool,
	})
	httpServer, err := newHTTPServer(ctx, cfg, authContext, cfg.GetHostAddress(), grpc.WithTransportCredentials(dialCreds))
	if err != nil {
		return err
	}

	conn, err := net.Listen("tcp", cfg.GetHostAddress())
	if err != nil {
		panic(err)
	}

	srv := &http.Server{
		Addr:    cfg.GetHostAddress(),
		Handler: grpcHandlerFunc(grpcServer, httpServer),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{*cert},
			NextProtos:   []string{"h2"},
		},
	}

	err = srv.Serve(tls.NewListener(conn, srv.TLSConfig))

	if err != nil {
		return errors.Wrapf(err, "failed to Start HTTP/2 Server")
	}
	return nil
}
