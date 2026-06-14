// Command seeddemo provisions one seeded demo organization directly against the
// database and prints an admin token + domain as JSON. A dev convenience for
// generating populated marketing screenshots when the running API predates the
// /demo endpoint; it signs the token with JWT_SECRET so that API still accepts it.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"b2bcommerce/internal/auth"
	"b2bcommerce/internal/db"
	"b2bcommerce/internal/demo"
)

func main() {
	ctx := context.Background()
	pool, err := db.NewPool(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "db:", err)
		os.Exit(1)
	}
	defer pool.Close()

	issuer := auth.NewIssuer(os.Getenv("JWT_SECRET"), 24*time.Hour)
	res, err := demo.Provision(ctx, pool, issuer, demo.Input{
		Email: "demo@teggo.dev", Company: "Northwind Industrial", Name: "Demo",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "provision:", err)
		os.Exit(1)
	}
	out, _ := json.Marshal(map[string]string{"token": res.Token, "domain": res.Domain})
	fmt.Println(string(out))
}
