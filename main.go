package main

import (
	"context"
	"encoding/json"
	_ "expvar"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var httpClient = http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

const serviceName = "check-resource-access"

// LookupPair contains the subject and resource for the permissions check.
type LookupPair struct {
	Subject  string `json:"subject"`
	Resource string `json:"resource"`
}

func main() {
	var (
		err          error
		permsURL     = flag.String("permissions-url", "http://permissions", "The URL for the permissions service.")
		resourceType = flag.String("resource-type", "analysis", "The type of resource to perform lookups for.")
		subjectType  = flag.String("subject-type", "user", "The subject type for lookups.")
		listenPort   = flag.Int("listen-port", 60000, "The port to listen on.")
		sslCert      = flag.String("ssl-cert", "", "Path to the SSL .crt file.")
		sslKey       = flag.String("ssl-key", "", "Path to the SSL .key file.")
	)

	flag.Parse()

	tracerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shutdown := tracerProviderFromEnv(tracerCtx, serviceName, func(e error) { log.Fatal(e) })
	defer shutdown()

	useSSL := false
	if *sslCert != "" || *sslKey != "" {
		if *sslCert == "" {
			log.Fatal("--ssl-cert is required with --ssl-key.")
		}

		if *sslKey == "" {
			log.Fatal("--ssl-key is required with --ssl-cert.")
		}
		useSSL = true
	}

	handler := otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var rb []byte
		rb, err = ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		lookup := &LookupPair{}

		if err = json.Unmarshal(rb, lookup); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if lookup.Subject == "" {
			http.Error(w, "subject name was empty", http.StatusBadRequest)
			return
		}

		if lookup.Resource == "" {
			http.Error(w, "resource was empty", http.StatusBadRequest)
			return
		}

		requrl, err := url.Parse(*permsURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		requrl.Path = filepath.Join(requrl.Path, "permissions/subjects", *subjectType, lookup.Subject, *resourceType, lookup.Resource)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requrl.String(), nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprint(w, string(b))
	}), "/")

	http.Handle("/", handler)

	addr := fmt.Sprintf(":%d", *listenPort)
	if useSSL {
		log.Fatal(http.ListenAndServeTLS(addr, *sslCert, *sslKey, nil))
	} else {
		log.Fatal(http.ListenAndServe(addr, nil))
	}
}
