package main

import (
	"encoding/json"
	_ "expvar"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
)

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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
		resp, err := http.Get(requrl.String())
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
		defer resp.Body.Close()

		fmt.Fprint(w, string(b))
	})

	addr := fmt.Sprintf(":%d", *listenPort)
	if useSSL {
		log.Fatal(http.ListenAndServeTLS(addr, *sslCert, *sslKey, nil))
	} else {
		log.Fatal(http.ListenAndServe(addr, nil))
	}
}
