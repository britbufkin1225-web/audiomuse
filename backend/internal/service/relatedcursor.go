package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// Phase 2D: the related-knowledge continuation contract.
//
// Phase 2A cut a discovery result at a documented bound and reported the eligible total beside
// it, so a caller could see what was left without being able to reach it. That was the right
// first answer - a reader choosing where to go next is not working through a result set - but it
// left one legitimate request unserved: a client rendering the whole of a well-connected record's
// neighbourhood had to raise the limit to the ceiling and then had nothing. This file is the
// bounded way to ask for the rest.
//
// It is a continuation cursor and nothing else. There is no session, no server-side result set,
// no cache, no database row and no process-local state of any kind: a continuation request is
// re-validated, re-scoped, re-scanned, re-grouped and re-ranked from the same immutable startup
// index the first page was built from, and the token only says where in that rebuilt ordering to
// resume. Two consequences follow and both are the point. A token survives a restart, because
// nothing it names was ever held in memory. And a token cannot be used to see anything a fresh
// request could not see, because every scope it carries is checked against the request that
// presents it rather than trusted from the token.
//
// The token is not authentication and it is not authorization. It is an opaque encoding of a
// position in a public, read-only ordering, on an API that has no identities to distinguish and
// no records to withhold. Nothing about it should be read as a capability: possessing one grants
// exactly what re-sending the original query string grants.
//
// It is deliberately not signed. The repository has no secret-management contract, no key
// material and no deployment step that could supply one, and an unkeyed digest carried alongside
// the payload it digests is not tamper protection - it detects corruption in transit that base64
// and the strict decode below already detect, while looking to a reader like something stronger.
// What actually protects the contract is that no field in the token is trusted on its own: the
// start, both scopes and the limit must equal what the presenting request independently resolves,
// and the cursor must name a record that the rebuilt ordering actually contains at the rank the
// token claims. An attacker editing a decoded payload can therefore only produce a token that is
// refused, or one that is identical in effect to a query string they could have written anyway.

// Continuation bounds.
//
// These are service constants for the reason every other related-knowledge bound is one: they are
// API safety invariants rather than deployment choices. A caller able to raise the encoded
// ceiling could hand the decoder an arbitrarily long string to walk before any of it was known to
// be a token at all.
const (
	// RelatedContinuationVersion is the token contract version this build issues.
	//
	// It is checked before any other field, so a token issued by a future contract is refused as
	// an unsupported version rather than being decoded as though its fields meant what this
	// build's fields mean. A version is carried rather than inferred from the payload shape
	// because a shape can be guessed at and a version cannot.
	RelatedContinuationVersion = 1

	// MaxRelatedContinuationTokenChars bounds the encoded token before it is decoded.
	//
	// It is the first check the decoder makes, so an oversized string costs one length comparison
	// rather than a base64 pass over whatever the caller sent. The number is roughly a third
	// larger than the longest token this build can issue - a maximal start identifier, a maximal
	// cursor identifier, both scopes named in full, at the ceiling limit - so a legitimate token
	// is never near it and no growth in the two closed vocabularies can push one over it without
	// this constant being revisited deliberately.
	MaxRelatedContinuationTokenChars = 1024

	// maxRelatedContinuationPayloadBytes bounds the decoded payload.
	//
	// Base64 already makes this arithmetic: a string of at most MaxRelatedContinuationTokenChars
	// characters cannot decode to more than three quarters of that. The check is written out
	// anyway so that the decoded bound is a stated property of the contract rather than a
	// consequence of the encoded one, and so that raising the encoded ceiling later cannot
	// silently unbound the structure the JSON decoder is asked to walk.
	maxRelatedContinuationPayloadBytes = MaxRelatedContinuationTokenChars * 3 / 4
)

// Continuation errors.
//
// They are distinct values rather than one refusal because each renders into a message naming the
// thing the caller must change, and the changes are not the same: a token that will never work
// anywhere, a token issued by a contract this build does not implement, a token presented against
// a different request, and a token whose position the current corpus no longer contains are four
// different situations for the client holding it.
//
// The malformed case is deliberately one error covering every structural fault - a token that is
// too long, that is not base64, that does not decode to UTF-8, that is not an object, that
// carries an unknown field, that omits a required one, that gives a field the wrong type, or that
// puts an out-of-range value in one. A caller cannot act differently on any of them, and
// separating them would publish a description of the decoder to the one input an attacker fully
// controls.
var (
	// ErrRelatedContinuationMalformed reports a token this build cannot read at all.
	ErrRelatedContinuationMalformed = errors.New("continuation token is malformed")

	// ErrRelatedContinuationVersion reports a token whose contract version this build does not
	// implement. It is separate from the malformed case because the token may be perfectly well
	// formed under a contract that is simply not this one, and the client's remedy - start the
	// traversal again rather than repair the token - is different.
	ErrRelatedContinuationVersion = errors.New("continuation token version is not supported")

	// ErrRelatedContinuationStart reports a token presented against a different start record.
	ErrRelatedContinuationStart = errors.New("continuation token was issued for a different start record")

	// ErrRelatedContinuationEntityScope reports a token presented with a different destination
	// scope. The comparison is over the normalised set, so a caller who reordered their own
	// entity_types list or wrote it in a different order between pages is not refused.
	ErrRelatedContinuationEntityScope = errors.New("continuation token was issued for a different entity_types scope")

	// ErrRelatedContinuationRelationshipScope reports a token presented with a different
	// relationship scope, under the same normalised-set comparison.
	ErrRelatedContinuationRelationshipScope = errors.New("continuation token was issued for a different relationship_types scope")

	// ErrRelatedContinuationLimit reports a token presented with a different effective limit.
	ErrRelatedContinuationLimit = errors.New("continuation token was issued for a different limit")

	// ErrRelatedContinuationCursor reports a token whose position the rebuilt result set does not
	// contain, which is what a token issued against a corpus that has since changed becomes. It
	// is refused rather than approximated: resuming from the nearest surviving position would
	// silently skip or repeat records, and a caller has no way to tell that happened.
	ErrRelatedContinuationCursor = errors.New("continuation token position is not present in this result set")
)

// relatedCursor is the decoded continuation payload.
//
// The JSON names are short because every byte of them is carried in a URL, and the field order is
// the declaration order, which is what makes the encoding deterministic: encoding/json writes a
// struct's fields in declaration order and nothing here is a map, so one request and one page
// produce one token on every run and in every process.
//
// It carries the minimum needed to resume safely and nothing else. There are no items in it, no
// evidence, no titles, no summaries, no counts and no scan statistics: a token that carried a
// result would be a result set the client could edit, and everything a page contains is rebuilt
// from the index anyway. There is no filesystem path, no repository internal and no server
// identifier, so a decoded token describes a request rather than the machine that answered it.
type relatedCursor struct {
	// Version is the token contract version. It is first in declaration order so that a decoded
	// payload's version is the first thing a reader of the wire format sees.
	Version int `json:"v"`

	// StartType and StartID are the route address the token was issued for. Both are compared
	// against the presenting request, so a token cannot be walked onto another record.
	StartType string `json:"st"`
	StartID   string `json:"si"`

	// EntityScope and RelationshipScope are the two normalised filters the first page applied,
	// each absent when that axis was unrestricted. They are stored normalised rather than as the
	// caller wrote them, so the compatibility check is over the set that actually ran.
	EntityScope       []string `json:"es,omitempty"`
	RelationshipScope []string `json:"rs,omitempty"`

	// Limit is the effective page limit, after the default and the ceiling have been applied. It
	// is bound into the token so that a continuation request cannot change the window size
	// mid-traversal, which would make "the next page" mean something different from the page the
	// token was issued beside.
	Limit int `json:"li"`

	// CursorType and CursorID are the identity of the last item the issuing page returned, and
	// they are the resume position. The pair is unique within one ordered result - a destination
	// is grouped exactly once - so it names a position unambiguously even where many items share
	// a rank, which an index into the list could not do across a corpus change.
	CursorType string `json:"ct"`
	CursorID   string `json:"ci"`

	// CursorRank and CursorDerived are the two ranking keys the cursor item held when the token
	// was issued, carried as a consistency check rather than as part of the address. If the
	// rebuilt ordering contains that record at a different precedence or direction, the ordering
	// the token was cut from is not the ordering being resumed, and the token is refused instead
	// of resuming into a list that has moved underneath it.
	CursorRank    int  `json:"cp"`
	CursorDerived bool `json:"cd"`
}

// newRelatedCursor builds the token payload for the page that ends at last.
func newRelatedCursor(start searchRef, scope searchScope, relationScope relatedScope, limit int, last domain.RelatedItem) relatedCursor {
	return relatedCursor{
		Version:           RelatedContinuationVersion,
		StartType:         string(start.entityType),
		StartID:           start.id,
		EntityScope:       scope.names(),
		RelationshipScope: relationScope.names(),
		Limit:             limit,
		CursorType:        string(last.EntityType),
		CursorID:          last.ID,
		CursorRank:        last.Reason.PriorityRank,
		CursorDerived:     last.Reason.Derived,
	}
}

// encode renders the payload as one opaque, URL-safe token.
//
// Raw base64url is used rather than the padded alphabet: the padding character is reserved in a
// query string and would have to be escaped by every client, and the length is already known from
// the string itself. The result is deterministic because the payload is a struct of scalars and
// ordered slices, so the same page of the same request encodes to the same bytes every time -
// which is what lets the whole traversal, tokens included, be asserted byte for byte.
func (c relatedCursor) encode() (string, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// decodeRelatedCursor turns a caller-supplied string into a validated payload.
//
// Every check here is structural: it asks whether this string is a token this build issued the
// shape of, and never whether it matches the request presenting it. That separation is what makes
// the refusal order deterministic - a token that is both malformed and for the wrong record is
// reported as malformed on every run - and it keeps the request comparison, which needs the
// resolved scopes, out of a function that needs nothing but the string.
//
// The checks run cheapest first, and the ordering is the bound as much as the sequence: length
// before base64, base64 before UTF-8, UTF-8 before the JSON walk, so no fault costs more work
// than the cheapest check that could have caught it. Unknown fields are refused rather than
// ignored, because a field this build does not know is a field some other contract meant
// something by, and silently dropping it would resume from a position whose meaning was written
// by a contract this build is not implementing.
//
// Nothing in the payload is used to select code, open a file, or reach a map before it has been
// range-checked. The two class names are compared against the closed vocabularies rather than
// looked up, so an arbitrary string in either can only be refused. The whole function returns one
// of two errors and never wraps the decoder's own, so a hostile token learns nothing about the
// decoder from the response it receives.
func decodeRelatedCursor(token string) (relatedCursor, error) {
	if token == "" || len(token) > MaxRelatedContinuationTokenChars {
		return relatedCursor{}, ErrRelatedContinuationMalformed
	}
	payload, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return relatedCursor{}, ErrRelatedContinuationMalformed
	}
	if len(payload) == 0 || len(payload) > maxRelatedContinuationPayloadBytes {
		return relatedCursor{}, ErrRelatedContinuationMalformed
	}
	if !utf8.Valid(payload) {
		return relatedCursor{}, ErrRelatedContinuationMalformed
	}
	// The payload must be a JSON object. The struct decode below already refuses an array, a
	// string and a number, but it accepts a bare null as a no-op that leaves every field zero -
	// which would then be reported as an unsupported version rather than as the malformed token it
	// is. Requiring the object shape up front makes "this is not a token" one answer rather than
	// two, and it is one byte comparison.
	body := strings.TrimSpace(string(payload))
	if body == "" || body[0] != '{' {
		return relatedCursor{}, ErrRelatedContinuationMalformed
	}

	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()
	var cursor relatedCursor
	if err := decoder.Decode(&cursor); err != nil {
		return relatedCursor{}, ErrRelatedContinuationMalformed
	}
	// A payload carrying a second JSON value after the object is refused whole. It is not a token
	// this build issued, and reading only the first value would accept a string whose remainder
	// no contract accounts for.
	if decoder.More() {
		return relatedCursor{}, ErrRelatedContinuationMalformed
	}

	// The version is checked before any other field is trusted, so a future contract's payload is
	// never interpreted under this one's field meanings.
	if cursor.Version != RelatedContinuationVersion {
		return relatedCursor{}, ErrRelatedContinuationVersion
	}
	if err := cursor.validate(); err != nil {
		return relatedCursor{}, err
	}
	return cursor, nil
}

// validate range-checks a decoded payload against the closed vocabularies and the declared bounds.
//
// Every field is checked, including the ones the compatibility test will check again against the
// request. The duplication is deliberate: this function's contract is that a payload leaving it is
// well formed on its own terms, so a later reader of the cursor cannot be handed a class name that
// is not a class or a limit that is outside the range the API documents. Nothing downstream has to
// re-establish that.
func (c relatedCursor) validate() error {
	if c.StartType == "" || len(c.StartType) > maxRelatedCursorFieldChars || !domain.ValidSearchEntityType(c.StartType) {
		return ErrRelatedContinuationMalformed
	}
	if c.StartID == "" || len(c.StartID) > maxRelatedCursorFieldChars {
		return ErrRelatedContinuationMalformed
	}
	if c.CursorType == "" || len(c.CursorType) > maxRelatedCursorFieldChars || !domain.ValidSearchEntityType(c.CursorType) {
		return ErrRelatedContinuationMalformed
	}
	if c.CursorID == "" || len(c.CursorID) > maxRelatedCursorFieldChars {
		return ErrRelatedContinuationMalformed
	}
	// The limit is range-checked rather than clamped. A clamp would let a token whose limit this
	// build would never have issued resume anyway, at a window size the issuing page did not use.
	if c.Limit < 1 || c.Limit > MaxRelatedLimit {
		return ErrRelatedContinuationMalformed
	}
	if c.CursorRank < 0 || c.CursorRank >= len(domain.RelatedPriorities) {
		return ErrRelatedContinuationMalformed
	}
	if !validRelatedCursorScope(c.EntityScope, len(domain.SearchEntityTypes), domain.ValidSearchEntityType) {
		return ErrRelatedContinuationMalformed
	}
	if !validRelatedCursorScope(c.RelationshipScope, len(domain.RelatedPriorities), domain.ValidRelatedPriority) {
		return ErrRelatedContinuationMalformed
	}
	return nil
}

// maxRelatedCursorFieldChars bounds each identifier inside a decoded payload.
//
// It is the same number the HTTP layer bounds a path identifier at, and deliberately so: a cursor
// names records that were reached through those routes, so a token carrying a longer identifier
// than any route accepts is naming something this API could not have addressed.
const maxRelatedCursorFieldChars = 128

// validRelatedCursorScope checks one decoded scope list against its closed vocabulary.
//
// A scope longer than its vocabulary, carrying a value outside it, or repeating one, is refused.
// The repetition check matters even though a repeated class cannot change which relations are
// admitted: an issued token carries a normalised set, so a payload that is not one was not issued
// by this build, and the compatibility comparison below is written against a normalised list.
func validRelatedCursorScope(values []string, max int, valid func(string) bool) bool {
	if len(values) > max {
		return false
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if value == "" || len(value) > maxRelatedCursorFieldChars || !valid(value) || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

// compatible reports whether a decoded token may be used to continue this request.
//
// The comparison is against what the presenting request independently resolved, never against
// what the token asserts, which is the whole of the token's security story: a token cannot widen
// a scope, move to another record, or change the window, because each of those is a field the
// request also determines and the two must agree. Nothing is repaired to make a token work, for
// the reason no filter on this route is repaired - a continuation that quietly answered a
// different question than the one asked would return a page a caller cannot tell apart from the
// page they meant.
//
// The checks are ordered from the address outwards - start, then destination scope, then
// relationship scope, then window - so a token presented against a wholly different request is
// always refused with the same error rather than with whichever mismatch happened to be tested
// first.
//
// Scopes are compared as ordered lists because both sides are normalised: the token carries the
// scope the issuing page applied in canonical order, and scope.names() renders the presenting
// request's in the same order. A caller who wrote their filter differently between pages, or
// wrote it in another order, therefore continues successfully, while a caller who actually
// changed the set does not.
func (c relatedCursor) compatible(start searchRef, scope searchScope, relationScope relatedScope, limit int) error {
	if c.StartType != string(start.entityType) || c.StartID != start.id {
		return ErrRelatedContinuationStart
	}
	if !equalStringSlices(c.EntityScope, scope.names()) {
		return ErrRelatedContinuationEntityScope
	}
	if !equalStringSlices(c.RelationshipScope, relationScope.names()) {
		return ErrRelatedContinuationRelationshipScope
	}
	if c.Limit != limit {
		return ErrRelatedContinuationLimit
	}
	return nil
}

// equalStringSlices compares two normalised scope renderings. A nil scope and an empty one are the
// same unrestricted axis, which is what both the query contract and the JSON omitempty on the
// payload already mean, so length is compared before membership rather than identity.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// resumeAfter returns the index of the first item that follows the cursor in the rebuilt ordering.
//
// The lookup is by identity and the rank keys are then verified, and the two are separate on
// purpose. Identity answers where the position is; the rank keys answer whether the ordering the
// token was cut from is the ordering being resumed. A record that is still present but now ranks
// differently means the corpus changed between the two requests, and continuing across that would
// hand the caller a page that is neither the next page of the old ordering nor of the new one.
//
// The scan is linear over the eligible items, which the relation scan ceiling already bounds, and
// runs once per continuation request. A map would be a second index over data that is rebuilt per
// request anyway, and would have to be built by the same linear pass it saved.
func (c relatedCursor) resumeAfter(items []domain.RelatedItem) (int, error) {
	for i, item := range items {
		if string(item.EntityType) != c.CursorType || item.ID != c.CursorID {
			continue
		}
		if item.Reason.PriorityRank != c.CursorRank || item.Reason.Derived != c.CursorDerived {
			return 0, ErrRelatedContinuationCursor
		}
		return i + 1, nil
	}
	return 0, ErrRelatedContinuationCursor
}
