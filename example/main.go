package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"

	"github.com/jveski/coalmine"
)

var (
	addr         = flag.String("addr", ":8080", "address to listen on")
	kill         = flag.Bool("kill", false, "force disable the feature")
	featOverride = flag.Bool("feat-override", false, "force enable the feature")
	override     = flag.Bool("override", false, "force enable all features")
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

	if *kill {
		// TODO
	}

	// Force the feature on (useful in tests)
	if *featOverride {
		baseCtx = coalmine.WithFeatureOverride(baseCtx, myFeature, true)
	}

	// Or force all features on
	if *featOverride {
		baseCtx = coalmine.WithGlobalOverride(baseCtx, true)
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		// Set additional values scoped to this individual request
		ctx := coalmine.WithValue(r.Context(), customerIDKey, r.URL.Query().Get("customer"))
		fmt.Fprintf(w, "feature enabled: %t\n", myFeature.Enabled(ctx))
	}

	svr := http.Server{
		BaseContext: func(net.Listener) context.Context { return baseCtx },
		Handler:     http.HandlerFunc(handler),
		Addr:        *addr,
	}
	panic(svr.ListenAndServe())
}
