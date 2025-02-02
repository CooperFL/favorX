package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gauss-project/aurorafs/pkg/aurora"

	"github.com/gauss-project/aurorafs/pkg/auth"

	"github.com/gauss-project/aurorafs/pkg/boson"
	"github.com/gauss-project/aurorafs/pkg/jsonhttp"
	"github.com/gauss-project/aurorafs/pkg/logging/httpaccess"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"resenje.org/web"
)

func (s *server) setupRouting() {
	const (
		apiVersion = "v1" // Only one api version exists, this should be configurable with more.
		rootPath   = "/" + apiVersion
	)

	router := mux.NewRouter()

	handle := func(path string, handler http.Handler) {
		if s.Restricted {
			handler = web.ChainHandlers(auth.PermissionCheckHandler(s.auth), web.FinalHandler(handler))
		}
		router.Handle(path, handler)
		router.Handle(rootPath+path, handler)
	}

	router.NotFoundHandler = http.HandlerFunc(jsonhttp.NotFoundHandler)

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Ethereum Boson favorX")
	})

	router.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "User-agent: *\nDisallow: /")
	})

	router.PathPrefix(aurora.RelayPrefixHttp + "/{gname}/{domain}").HandlerFunc(s.relayDo)

	if s.Restricted {
		router.Handle("/auth", jsonhttp.MethodHandler{
			"POST": web.ChainHandlers(
				s.newTracingHandler("auth"),
				jsonhttp.NewMaxBodyBytesHandler(512),
				web.FinalHandlerFunc(s.authHandler),
			),
		})
		router.Handle("/refresh", jsonhttp.MethodHandler{
			"POST": web.ChainHandlers(
				s.newTracingHandler("auth"),
				jsonhttp.NewMaxBodyBytesHandler(512),
				web.FinalHandlerFunc(s.refreshHandler),
			),
		})
	}

	handle("/apiPort", jsonhttp.MethodHandler{
		"GET": http.HandlerFunc(s.apiPort),
	})

	handle("/restore", jsonhttp.MethodHandler{
		"GET": http.HandlerFunc(s.restore),
	})

	handle("/bytes", jsonhttp.MethodHandler{
		"POST": web.ChainHandlers(
			s.newTracingHandler("bytes-upload"),
			web.FinalHandlerFunc(s.bytesUploadHandler),
		),
	})

	handle("/bytes/{address}", jsonhttp.MethodHandler{
		"GET": web.ChainHandlers(
			s.newTracingHandler("bytes-download"),
			web.FinalHandlerFunc(s.bytesGetHandler),
		),
	})

	handle("/chunks", jsonhttp.MethodHandler{
		"POST": web.ChainHandlers(
			jsonhttp.NewMaxBodyBytesHandler(boson.ChunkWithSpanSize),
			web.FinalHandlerFunc(s.chunkUploadHandler),
		),
	})

	handle("/chunks/{addr}", jsonhttp.MethodHandler{
		"GET": http.HandlerFunc(s.chunkGetHandler),
	})

	handle("/soc/{owner}/{id}", jsonhttp.MethodHandler{
		"POST": web.ChainHandlers(
			jsonhttp.NewMaxBodyBytesHandler(boson.ChunkWithSpanSize),
			web.FinalHandlerFunc(s.socUploadHandler),
		),
	})

	handle("/file", jsonhttp.MethodHandler{
		"GET": web.ChainHandlers(
			s.newTracingHandler("file-list"),
			web.FinalHandlerFunc(s.auroraListHandler),
		),
		"POST": web.ChainHandlers(
			s.newTracingHandler("file-upload"),
			web.FinalHandlerFunc(s.auroraUploadHandler),
		),
	})

	handle("/file/{address}", jsonhttp.MethodHandler{
		"GET": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := r.URL
			u.Path += "/"
			http.Redirect(w, r, u.String(), http.StatusPermanentRedirect)
		}),
		"DELETE": web.ChainHandlers(
			s.newTracingHandler("file-delete"),
			web.FinalHandlerFunc(s.auroraDeleteHandler),
		),
	})

	handle("/file/{address}/{path:.*}", jsonhttp.MethodHandler{
		"GET": web.ChainHandlers(
			s.newTracingHandler("file-download"),
			web.FinalHandlerFunc(s.auroraDownloadHandler),
		),
	})

	handle("/manifest/{address}", jsonhttp.MethodHandler{
		"GET": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := r.URL
			u.Path += "/"
			http.Redirect(w, r, u.String(), http.StatusPermanentRedirect)
		}),
	})

	handle("/manifest/{address}/{path:.*}", jsonhttp.MethodHandler{
		"GET": web.ChainHandlers(
			s.newTracingHandler("manifest-view"),
			web.FinalHandlerFunc(s.manifestViewHandler),
		),
	})

	handle("/pins", web.ChainHandlers(
		s.gatewayModeForbidEndpointHandler,
		web.FinalHandler(jsonhttp.MethodHandler{
			"GET": http.HandlerFunc(s.listPinnedRootHashes),
		})),
	)

	handle("/pins/{reference}", web.ChainHandlers(
		s.gatewayModeForbidEndpointHandler,
		web.FinalHandler(jsonhttp.MethodHandler{
			"GET":    http.HandlerFunc(s.getPinnedRootHash),
			"POST":   http.HandlerFunc(s.pinRootHash),
			"DELETE": http.HandlerFunc(s.unpinRootHash),
		})),
	)

	handle("/traffic/info", web.ChainHandlers(
		s.gatewayModeForbidEndpointHandler,
		web.FinalHandler(jsonhttp.MethodHandler{
			"GET": http.HandlerFunc(s.trafficInfo),
		})),
	)

	handle("/traffic/address", web.ChainHandlers(
		s.gatewayModeForbidEndpointHandler,
		web.FinalHandler(jsonhttp.MethodHandler{
			"GET": http.HandlerFunc(s.address),
		})),
	)

	handle("/traffic/cheques", web.ChainHandlers(
		s.gatewayModeForbidEndpointHandler,
		web.FinalHandler(jsonhttp.MethodHandler{
			"GET": http.HandlerFunc(s.trafficCheques),
		})),
	)

	handle("/traffic/cash/{address}", web.ChainHandlers(
		s.gatewayModeForbidEndpointHandler,
		web.FinalHandler(jsonhttp.MethodHandler{
			"POST": http.HandlerFunc(s.cashCheque),
		})),
	)

	handle("/fileRegister/{address}", jsonhttp.MethodHandler{
		"POST": web.ChainHandlers(
			s.newTracingHandler("aurora-Register"),
			web.FinalHandlerFunc(s.fileRegister),
		),
		"DELETE": web.ChainHandlers(
			s.newTracingHandler("aurora-RegisterRemove"),
			web.FinalHandlerFunc(s.fileRegisterRemove),
		),
	})

	handle("/group/join/{gid}", jsonhttp.MethodHandler{
		"POST":   http.HandlerFunc(s.groupJoinHandler),
		"DELETE": http.HandlerFunc(s.groupLeaveHandler),
	})
	handle("/group/observe/{gid}", jsonhttp.MethodHandler{
		"POST":   http.HandlerFunc(s.groupObserveHandler),
		"DELETE": http.HandlerFunc(s.groupObserveCancelHandler),
	})
	handle("/group/multicast/{gid}", jsonhttp.MethodHandler{
		"POST": http.HandlerFunc(s.multicastMsg),
	})
	handle("/group/notify/{gid}/{target}", jsonhttp.MethodHandler{
		"POST": http.HandlerFunc(s.notify),
	})
	handle("/group/send/{gid}/{target}", jsonhttp.MethodHandler{
		"POST": http.HandlerFunc(s.sendReceive),
	})
	handle("/group/peers/{gid}", jsonhttp.MethodHandler{
		"GET": http.HandlerFunc(s.peers),
	})

	s.newLoopbackRouter(router)

	s.Handler = web.ChainHandlers(
		httpaccess.NewHTTPAccessLogHandler(s.logger, logrus.InfoLevel, s.tracer, "api access"),
		handlers.CompressHandler,
		s.responseCodeMetricsHandler,
		s.pageviewMetricsHandler,
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if o := r.Header.Get("Origin"); o != "" && s.checkOrigin(r) {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Allow-Origin", o)
					w.Header().Set("Access-Control-Allow-Headers", "User-Agent, Origin, Accept, Authorization, Content-Type, X-Requested-With, Access-Control-Request-Headers, Access-Control-Request-Method, Aurora-Tag, Aurora-Pin, Aurora-Encrypt, Aurora-Index-Document, Aurora-Error-Document, Aurora-Collection, Aurora-Collection-Name")
					w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS, POST, PUT, DELETE")
					w.Header().Set("Access-Control-Max-Age", "3600")
				}
				h.ServeHTTP(w, r)
			})
		},
		s.gatewayModeForbidHeadersHandler,
		web.FinalHandler(router),
	)
}
func (s *server) newLoopbackRouter(router *mux.Router) {
	var handle = func(path string, handler http.Handler) {
		handler = web.ChainHandlers(
			// auth.AllowLoopbackIP(),
			web.FinalHandler(handler),
		)
		router.Handle(path, handler)
	}

	handle("/chain", web.ChainHandlers(
		web.FinalHandler(jsonhttp.MethodHandler{
			"POST": http.HandlerFunc(s.chainHandler),
			"GET":  http.HandlerFunc(s.chainTransactionHandler),
		})),
	)

}
func (s *server) gatewayModeForbidEndpointHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.GatewayMode {
			s.logger.Tracef("gateway mode: forbidden %s", r.URL.String())
			jsonhttp.Forbidden(w, nil)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *server) gatewayModeForbidHeadersHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.GatewayMode {
			if strings.ToLower(r.Header.Get(AuroraPinHeader)) == "true" {
				s.logger.Tracef("gateway mode: forbidden pinning %s", r.URL.String())
				jsonhttp.Forbidden(w, "pinning is disabled")
				return
			}
			if strings.ToLower(r.Header.Get(AuroraEncryptHeader)) == "true" {
				s.logger.Tracef("gateway mode: forbidden encryption %s", r.URL.String())
				jsonhttp.Forbidden(w, "encryption is disabled")
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}
