package httpapi

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/helmnet/beer-menu/api/internal/importer"
	"github.com/helmnet/beer-menu/api/internal/kegs"
)

// routeFixture builds the rows one request to a route needs and returns a concrete path and a
// valid body for it, so the request succeeds with a write token or a session.
type routeFixture func(t *testing.T, deps Deps) (path, body string)

// routeFixtures holds a fixture for every AccessRead and AccessWrite route, keyed by
// "<METHOD> <pattern>". A route without one fails the walk tests, so a new route cannot skip
// the credential checks.
var routeFixtures = map[string]routeFixture{
	"GET /api/beers": func(*testing.T, Deps) (string, string) { return "/api/beers", "" },
	"GET /api/beers/{id}": func(t *testing.T, deps Deps) (string, string) {
		return "/api/beers/" + insertBeer(t, deps, "Pale"), ""
	},
	"GET /api/brewfather": func(*testing.T, Deps) (string, string) { return "/api/brewfather", "" },
	"GET /api/brewfather/batches": func(t *testing.T, deps Deps) (string, string) {
		connectFake(t, deps)
		return "/api/brewfather/batches", ""
	},
	"POST /api/brewfather/imports": func(t *testing.T, deps Deps) (string, string) {
		ref := "batch-" + rand.Text()
		connectFake(t, deps).SetBatches(importer.Batch{Ref: ref, Name: "Imported", Status: "fermenting"})
		return "/api/brewfather/imports", `{"batch_ids":["` + ref + `"]}`
	},
	"POST /api/beers/{id}/sync": func(t *testing.T, deps Deps) (string, string) {
		ref := "batch-" + rand.Text()
		connectFake(t, deps).SetBatches(importer.Batch{Ref: ref, Name: "Imported", Status: "fermenting"})
		return "/api/beers/" + importFake(t, deps, ref) + "/sync", ""
	},
	// Each request starts a background sync. Wait for the last one so the next is not refused as
	// running, and at cleanup so no sync changes data under another route's walk.
	"POST /api/brewfather/sync": func(t *testing.T, deps Deps) (string, string) {
		deps.Importer.Wait()
		t.Cleanup(deps.Importer.Wait)
		connectFake(t, deps)
		return "/api/brewfather/sync", ""
	},
	"GET /api/settings":   func(*testing.T, Deps) (string, string) { return "/api/settings", "" },
	"PATCH /api/settings": func(*testing.T, Deps) (string, string) { return "/api/settings", `{"title":"Walk"}` },
	"GET /api/srm-colors": func(*testing.T, Deps) (string, string) { return "/api/srm-colors", "" },
	"GET /api/keg-types":  func(*testing.T, Deps) (string, string) { return "/api/keg-types", "" },
	"GET /api/taps":       func(*testing.T, Deps) (string, string) { return "/api/taps", "" },
	"GET /api/taps/{id}": func(t *testing.T, deps Deps) (string, string) {
		return "/api/taps/" + insertTap(t, deps, "Left"), ""
	},
	"POST /api/taps": func(*testing.T, Deps) (string, string) { return "/api/taps", `{"label":"x"}` },
	"PATCH /api/taps/{id}": func(t *testing.T, deps Deps) (string, string) {
		return "/api/taps/" + insertTap(t, deps, "Left"), `{"enabled":false}`
	},
	"PUT /api/taps/order": func(t *testing.T, deps Deps) (string, string) {
		list, err := kegs.ListTaps(deps.DB)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(list))
		for i, tp := range list {
			ids[len(list)-1-i] = strconv.FormatInt(tp.ID, 10)
		}
		return "/api/taps/order", `{"ids":[` + strings.Join(ids, ",") + `]}`
	},
	"GET /api/kegs": func(*testing.T, Deps) (string, string) { return "/api/kegs", "" },
	"GET /api/kegs/{id}": func(t *testing.T, deps Deps) (string, string) {
		return "/api/kegs/" + insertKeg(t, deps, insertBeer(t, deps, "Pale"), ""), ""
	},
	"POST /api/kegs": func(t *testing.T, deps Deps) (string, string) {
		return "/api/kegs", `{"beer_id":` + insertBeer(t, deps, "Pale") + `,"tap_id":` + insertTap(t, deps, "Left") + `}`
	},
	"PATCH /api/kegs/{id}": func(t *testing.T, deps Deps) (string, string) {
		return "/api/kegs/" + insertKeg(t, deps, insertBeer(t, deps, "Pale"), ""), `{"tap_id":` + insertTap(t, deps, "Left") + `}`
	},
	"DELETE /api/kegs/{id}": func(t *testing.T, deps Deps) (string, string) {
		return "/api/kegs/" + insertKeg(t, deps, insertBeer(t, deps, "Pale"), ""), ""
	},
	"POST /api/kegs/{id}/pours": func(t *testing.T, deps Deps) (string, string) {
		return "/api/kegs/" + insertKeg(t, deps, insertBeer(t, deps, "Pale"), "") + "/pours", `{"amount_l":0.5}`
	},
	// The seeded tap stays, so the tap removed is never the last one.
	"DELETE /api/taps/{id}": func(t *testing.T, deps Deps) (string, string) {
		return "/api/taps/" + insertTap(t, deps, "Gone"), ""
	},
	"POST /api/beers": func(*testing.T, Deps) (string, string) { return "/api/beers", `{"name":"x"}` },
	"PATCH /api/beers/{id}": func(t *testing.T, deps Deps) (string, string) {
		return "/api/beers/" + insertBeer(t, deps, "Pale"), `{"abv":1}`
	},
	"DELETE /api/beers/{id}": func(t *testing.T, deps Deps) (string, string) {
		return "/api/beers/" + insertBeer(t, deps, "Gone"), ""
	},
}

// walkedRoutes returns the routes in Routes with access, each with its fixture, failing t for a
// route that has none.
func walkedRoutes(t *testing.T, access Access) map[string]routeFixture {
	t.Helper()
	out := map[string]routeFixture{}
	for _, r := range Routes {
		if r.Access != access {
			continue
		}
		name := r.Method + " " + r.Path
		fx, ok := routeFixtures[name]
		if !ok {
			t.Errorf("%s: no entry in routeFixtures", name)
			continue
		}
		out[name] = fx
	}
	return out
}

// TestRouteFixturesMatchRoutes fails for a fixture naming no read or write route, such as one
// left behind after a route's access changed, which would otherwise drop it from the walk.
func TestRouteFixturesMatchRoutes(t *testing.T) {
	walked := walkedRoutes(t, AccessRead)
	for name, fx := range walkedRoutes(t, AccessWrite) {
		walked[name] = fx
	}
	for name := range routeFixtures {
		if _, ok := walked[name]; !ok {
			t.Errorf("routeFixtures has %s, which is not a read or write route", name)
		}
	}
}

// dataTables returns every row of the beers, taps and kegs tables as text, to prove a refused
// request changed nothing.
func dataTables(t *testing.T, deps Deps) string {
	t.Helper()
	var out strings.Builder
	for _, table := range []string{"beers", "taps", "kegs"} {
		rows, err := deps.DB.Query("SELECT * FROM " + table + " ORDER BY id")
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			fmt.Fprintln(&out, table, vals)
		}
		rows.Close()
	}
	return out.String()
}

// TestWritesRefuseWeakCredentials walks every AccessWrite route: no credential and a revoked
// token give 401, a read token and a cross-site session request give 403, none of them change
// any data, and a write token and a session each succeed on a fresh fixture (constitution IV).
// The cross-site case is skipped for GET write routes: http.CrossOriginProtection never blocks
// safe methods, and a GET changes nothing.
func TestWritesRefuseWeakCredentials(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	for name, fx := range walkedRoutes(t, AccessWrite) {
		method := strings.Fields(name)[0]
		t.Run(name, func(t *testing.T) {
			path, body := fx(t, deps)
			before := dataTables(t, deps)
			wantError(t, do(mux, method, path, body), 401, "unauthenticated")
			wantError(t, do(mux, method, path, body, c.read...), 403, "forbidden")
			wantError(t, do(mux, method, path, body, c.revoked...), 401, "unauthenticated")
			if method != http.MethodGet {
				wantError(t, do(mux, method, path, body, c.crossSite...), 403, "forbidden")
			}
			if after := dataTables(t, deps); after != before {
				t.Fatalf("a refused request changed data:\n%s\n%s", before, after)
			}
			for who, h := range map[string][]string{"write token": c.write, "session": c.session} {
				path, body := fx(t, deps)
				if rec := do(mux, method, path, body, h...); rec.Code < 200 || rec.Code > 299 {
					t.Errorf("%s %s with %s = %d %s, want 2xx", method, path, who, rec.Code, rec.Body)
				}
			}
		})
	}
}

// TestReadsNeedCredential walks every AccessRead route: no credential and a revoked token give
// 401, so a read cannot silently become public. Feature 5 adds the public menu endpoint.
func TestReadsNeedCredential(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	for name, fx := range walkedRoutes(t, AccessRead) {
		t.Run(name, func(t *testing.T) {
			path, _ := fx(t, deps)
			wantError(t, do(mux, http.MethodGet, path, ""), 401, "unauthenticated")
			wantError(t, do(mux, http.MethodGet, path, "", c.revoked...), 401, "unauthenticated")
		})
	}
	// The guard runs before the handler, so an unknown id is not told apart from a known one.
	for _, p := range []string{"/api/beers/999", "/api/beers/abc"} {
		wantError(t, do(mux, http.MethodGet, p, ""), 401, "unauthenticated")
	}
}

// TestReadsAnyCredential is the success side of TestReadsNeedCredential: every read route
// answers 200 to a read token, a write token and a session.
func TestReadsAnyCredential(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	for name, fx := range walkedRoutes(t, AccessRead) {
		t.Run(name, func(t *testing.T) {
			path, _ := fx(t, deps)
			for who, h := range map[string][]string{"read token": c.read, "write token": c.write, "session": c.session} {
				if rec := do(mux, http.MethodGet, path, "", h...); rec.Code != 200 {
					t.Errorf("GET %s with %s = %d %s", path, who, rec.Code, rec.Body)
				}
			}
		})
	}
}
