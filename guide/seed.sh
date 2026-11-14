#!/bin/sh
# Fills a fresh, throwaway local Beer Menu stack with invented sample data for the user guide's
# screenshots (specs/017-user-guide/research.md R5): a first admin, a second admin, a read token,
# beers, four taps, kegs on three of them, two waiting kegs and two kicked kegs.
# Usage: sh guide/seed.sh [BASE_URL]   (default http://localhost:8080)
# It refuses to run unless first-run setup is still needed, so it never writes to a real
# instance, and stops at the first failed request. POSIX sh and curl only; ids are read with sed.
# Every name and the password below are invented; the stack is deleted after the screenshots.
set -eu

BASE=${1:-http://localhost:8080}
ADMIN=brewer
PASSWORD=porch-light-sample-only

jar=$(mktemp)
trap 'rm -f "$jar"' EXIT

# api sends one request with the session cookie; --fail-with-body shows the API's message on error.
api() {
  method=$1 path=$2
  shift 2
  curl -sS --fail-with-body -b "$jar" -c "$jar" -X "$method" -H 'Content-Type: application/json' "$@" "$BASE$path"
}

# new POSTs the JSON body $2 to $1 and prints the created object's id.
new() {
  # Captured output would hide the API's message, so print it before stopping.
  out=$(api POST "$1" -d "$2") || { echo "POST $1 failed: $out" >&2; exit 1; }
  id=$(printf '%s' "$out" | sed -n 's/^{"id":\([0-9][0-9]*\).*/\1/p')
  if [ -z "$id" ]; then
    echo "no id in the reply from $1: $out" >&2
    exit 1
  fi
  echo "$id"
}

# beer adds one beer: name, style, ABV, IBU, SRM, description.
beer() {
  new /api/beers "{\"name\":\"$1\",\"style\":\"$2\",\"abv\":$3,\"ibu\":$4,\"srm\":$5,\"description\":\"$6\"}"
}

case $(curl -sS --fail-with-body "$BASE/api/setup") in
  *'"needed":true'*) ;;
  *)
    echo "setup already done; seed.sh only runs on a fresh stack" >&2
    exit 1
    ;;
esac

api POST /api/setup -d "{\"username\":\"$ADMIN\",\"password\":\"$PASSWORD\"}" >/dev/null

# Adding an admin or a token needs a recent sign-in; the setup session is one, so do these first.
api POST /api/admins -d '{"username":"cellar-helper","password":"cellar-helper-sample-only"}' >/dev/null
api POST /api/tokens -d '{"name":"Kitchen tablet","scope":"read"}' >/dev/null

pale=$(beer "Porch Light Pale Ale" "American Pale Ale" 5.4 38 6 "Bright citrus hops over a crisp, biscuity malt base.")
stout=$(beer "Backyard Oak Stout" "Oatmeal Stout" 6.2 32 38 "Smooth and roasty, with notes of coffee and dark chocolate.")
hefe=$(beer "Rainy Sunday Hefeweizen" "Hefeweizen" 5.0 12 4 "Banana and clove from a classic wheat beer yeast.")
amber=$(beer "Lantern Glow Amber" "Amber Ale" 5.6 25 14 "Toffee and caramel, finished with a gentle hop bite.")
session=$(beer "Hammock Session IPA" "Session IPA" 4.3 45 5 "All the hop aroma of an IPA in a lighter, easy-drinking beer.")
beer "Garden Gate Saison" "Saison" 6.5 28 5 "Peppery and dry, with a hint of orange peel." >/dev/null
pils=$(beer "Twilight Pilsner" "German Pilsner" 4.8 35 3 "Clean, crisp and floral.")
brown=$(beer "Maple Hollow Brown" "Brown Ale" 5.8 22 20 "Nutty and toasty, with a soft, sweet finish.")

# A fresh install has one tap; add three more, named by the API.
# No pipe: a pipeline's status is sed's, which would hide a failed request.
taps=$(api GET /api/taps)
tap1=$(printf '%s' "$taps" | sed -n 's/^\[{"id":\([0-9][0-9]*\).*/\1/p')
if [ -z "$tap1" ]; then
  echo "no tap in the reply from /api/taps: $taps" >&2
  exit 1
fi
tap2=$(new /api/taps '{}')
tap3=$(new /api/taps '{}')
tap4=$(new /api/taps '{}')

# Keg History: two kegs that ran on tap 4, then were kicked, leaving that tap empty.
for b in "$pils" "$brown"; do
  keg=$(new /api/kegs "{\"beer_id\":$b,\"type\":\"corny_5_gal\",\"tap_id\":$tap4}")
  api PATCH "/api/kegs/$keg" -d '{"kicked":true}' >/dev/null
done

# Taps: three kegs at different levels, so the level bars vary.
new /api/kegs "{\"beer_id\":$pale,\"type\":\"corny_5_gal\",\"remaining_l\":12.5,\"tap_id\":$tap1}" >/dev/null
new /api/kegs "{\"beer_id\":$stout,\"type\":\"corny_5_gal\",\"remaining_l\":5.2,\"tap_id\":$tap2}" >/dev/null
new /api/kegs "{\"beer_id\":$hefe,\"type\":\"sanke_sixth\",\"tap_id\":$tap3}" >/dev/null

# On Deck: two waiting kegs.
new /api/kegs "{\"beer_id\":$amber,\"type\":\"corny_5_gal\"}" >/dev/null
new /api/kegs "{\"beer_id\":$session,\"type\":\"corny_3_gal\"}" >/dev/null

cat <<EOF
Sample data added.
Admin app: $BASE/  (sign in as $ADMIN / $PASSWORD)
TV display: $BASE/display
EOF
