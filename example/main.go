package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/jveski/coalmine"
)

var (
	addr         = flag.String("addr", ":8080", "address to listen on")
	featOverride = flag.Bool("feat-override", false, "force enable the feature")
)

const (
	regionKey     coalmine.Key = "region"
	customerIDKey coalmine.Key = "customerID"
)

var (
	myFeature = coalmine.NewFeature("myFeature",
		// enable for 50% of customers in westus
		coalmine.WithAND(
			coalmine.WithExactMatch(regionKey, "westus"),
			coalmine.WithPercentage(customerIDKey, 50),
		),

		// enable for all customers in southcentralus
		coalmine.WithExactMatch(regionKey, "southcentralus"),
	)
)

func main() {
	flag.Parse()

	// Set values that live for the life of the service on the base context
	baseCtx := context.Background()
	baseCtx = coalmine.WithValue(baseCtx, regionKey, "westus")

	// Log feature states
	baseCtx = coalmine.WithObserver(baseCtx, func(ctx context.Context, feature string, state bool) {
		log.Printf("feature %q is enabled: %t", feature, state)
	})

	// Force the feature on (useful in tests)
	if *featOverride {
		baseCtx = coalmine.WithOverride(baseCtx, myFeature, true)
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		// Set additional values scoped to this individual request
		ctx := coalmine.WithValue(r.Context(), customerIDKey, r.URL.Query().Get("customer"))

		// Check the feature state
		enabled := myFeature.Enabled(ctx)
		fmt.Fprintf(w, "feature enabled: %t\n", enabled)
	}

	svr := http.Server{
		BaseContext: func(net.Listener) context.Context { return baseCtx },
		Handler:     http.HandlerFunc(handler),
		Addr:        *addr,
	}
	panic(svr.ListenAndServe())
}
