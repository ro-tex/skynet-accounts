package jwt

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"

	"github.com/dgrijalva/jwt-go"
	"github.com/lestrrat/go-jwx/jwk"
	"github.com/sirupsen/logrus"
	"gitlab.com/NebulousLabs/errors"
)

var (
	// oathkeeperPubKeys is the public RS key exposed by Oathkeeper for JWT
	// validation. It's available at oathkeeperPubKeyURL.
	oathkeeperPubKeys *jwk.Set = nil

	// KratosAddr holds the domain + port on which we can find Kratos.
	// The point of this var is to be overridable via .env.
	KratosAddr = "kratos:4433"

	// OathkeeperAddr holds the domain + port on which we can find Oathkeeper.
	// The point of this var is to be overridable via .env.
	OathkeeperAddr = "oathkeeper:4456"
)

// ctxValue is a helper type which makes it safe to register values in the
// context. If we don't use a custom unexported type it's easy for others
// to get our value or accidentally overwrite it.
type ctxValue string

// ContextWithToken returns a copy of the given context that contains a token.
func ContextWithToken(ctx context.Context, token *jwt.Token) context.Context {
	return context.WithValue(ctx, ctxValue("token"), token)
}

// TokenExpiration extracts and returns the `exp` claim of the given token.
// NOTE: It does NOT validate the token!
func TokenExpiration(t *jwt.Token) (int64, error) {
	if t == nil {
		return 0, errors.New("invalid token")
	}
	if reflect.ValueOf(t.Claims).Kind() != reflect.ValueOf(jwt.MapClaims{}).Kind() {
		return 0, errors.New("the token does not contain the claims we expect")
	}
	claims := t.Claims.(jwt.MapClaims)
	if reflect.ValueOf(claims["exp"]).Kind() != reflect.Float64 {
		return 0, errors.New("the token does not contain the claims we expect")
	}
	return int64(claims["exp"].(float64)), nil
}

// TokenFromContext extracts the JWT token from the
// context and returns the contained user sub, claims and the token itself.
// The sub is the user id used in Kratos.
//
// Example claims structure:
//
// map[
//    exp:1.607594172e+09
//    iat:1.607593272e+09
//    iss:https://siasky.net/
//    jti:1e5872ae-71d8-49ec-a550-4fc6163cbbf2
//    nbf:1.607593272e+09
//    sub:695725d4-a345-4e68-919a-7395cb68484c
//    session:map[
//        active:true
//        authenticated_at:2020-12-09T16:09:35.004003Z
//        issued_at:2020-12-09T16:09:35.004042Z
//        expires_at:2020-12-10T16:09:35.004003Z
//        id:9911ad26-e47f-4ec4-86a1-fbbc7fd5073e
//        identity:map[
//            id:695725d4-a345-4e68-919a-7395cb68484c
//            recovery_addresses:[
//                map[
//                    id:e2d847e1-1885-4edf-bccb-64b527b30096
//                    value:ivaylo@nebulous.tech
//                    via:email
//                ]
//            ]
//            schema_id:default
//            schema_url:https://siasky.net/secure/.ory/kratos/public/schemas/default
//            traits:map[
//                email:ivaylo@nebulous.tech
//                name:map[
//                    first:Ivaylo
//                    last:Novakov
//                ]—
//            ]
//            verifiable_addresses:[
//                map[
//                    id:953b0c1a-def9-4fa2-af23-fb36c00768d2
//                    status:pending
//                    value:ivaylo@nebulous.tech
//                    verified:true
//                    verified_at:2020-12-09T16:09:35.004042Z
//                    via:email
//                ]
//            ]
//        ]
//    ]
// ]
func TokenFromContext(ctx context.Context) (sub string, claims jwt.MapClaims, token *jwt.Token, err error) {
	t, ok := ctx.Value(ctxValue("token")).(*jwt.Token)
	if !ok {
		err = errors.New("failed to get token")
		return
	}
	if reflect.ValueOf(t.Claims).Kind() != reflect.ValueOf(jwt.MapClaims{}).Kind() {
		err = errors.New("the token does not contain the claims we expect")
		return
	}
	claims = t.Claims.(jwt.MapClaims)
	if reflect.ValueOf(claims["sub"]).Kind() != reflect.String {
		err = errors.New("the token does not contain the sub we expect")
		return
	}
	subEntry, ok := claims["sub"]
	if !ok {
		claims = nil
		err = errors.New("jwt claims don't contain a valid sub")
		return
	}
	sub = subEntry.(string)
	token = t
	return
}

// UserDetailsFromJWT extracts the user details from the JWT token embedded in
// the context. We do it that way, so we can call this from anywhere in the code.
func UserDetailsFromJWT(ctx context.Context) (firstName, lastName, email string, err error) {
	if ctx == nil {
		err = errors.New("Invalid context")
		return
	}
	_, claims, _, err := TokenFromContext(ctx)
	if err != nil {
		return
	}
	// Validate the chain of inset maps claims->session->identity->traits->name
	// and then extract the data we need.
	if reflect.ValueOf(claims["session"]).Kind() != reflect.Map {
		err = errors.New("the token does not contain the sessions we expect")
		return
	}
	session := claims["session"].(map[string]interface{})
	if reflect.ValueOf(session["identity"]).Kind() != reflect.Map {
		err = errors.New("the token does not contain the identity we expect")
		return
	}
	id := session["identity"].(map[string]interface{})
	if reflect.ValueOf(id["traits"]).Kind() != reflect.Map {
		err = errors.New("the token does not contain the traits we expect")
		return
	}
	tr := id["traits"].(map[string]interface{})
	if reflect.ValueOf(tr["name"]).Kind() != reflect.Map {
		err = errors.New("the token does not contain the names we expect")
		return
	}
	name := tr["name"].(map[string]interface{})
	if reflect.ValueOf(tr["email"]).Kind() != reflect.String {
		err = errors.New("the token does not contain the email we expect")
		return
	}
	email = tr["email"].(string)
	firstName = name["first"].(string)
	lastName = name["last"].(string)
	return
}

// ValidateToken verifies the validity of a JWT token, both in terms of validity
// of the signature and expiration time.
//
// Example token:
//
// Header:
//
//{
//  "alg": "RS256",
//  "kid": "a2aa9739-d753-4a0d-87ee-61f101050277",
//  "typ": "JWT"
//}
//
// Payload:
//
//{
//  "exp": 1607594172,
//  "iat": 1607593272,
//  "iss": "https://siasky.net/",
//  "jti": "1e5872ae-71d8-49ec-a550-4fc6163cbbf2",
//  "nbf": 1607593272,
//  "session": {
//    "active": true,
//    "authenticated_at": "2020-12-09T16:09:35.004003Z",
//    "expires_at": "2020-12-10T16:09:35.004003Z",
//    "id": "9911ad26-e47f-4ec4-86a1-fbbc7fd5073e",
//    "identity": {
//      "id": "695725d4-a345-4e68-919a-7395cb68484c",
//      "recovery_addresses": [
//        {
//          "id": "e2d847e1-1885-4edf-bccb-64b527b30096",
//          "value": "ivaylo@nebulous.tech",
//          "via": "email"
//        }
//      ],
//      "schema_id": "default",
//      "schema_url": "https://siasky.net/secure/.ory/kratos/public/schemas/default",
//      "traits": {
//        "email": "ivaylo@nebulous.tech",
//        "name": {
//          "first": "Ivaylo",
//          "last": "Novakov"
//        }
//      },
//      "verifiable_addresses": [
//        {
//          "id": "953b0c1a-def9-4fa2-af23-fb36c00768d2",
//          "status": "pending",
//          "value": "ivaylo@nebulous.tech",
//          "verified": false,
//          "verified_at": null,
//          "via": "email"
//        }
//      ]
//    },
//    "issued_at": "2020-12-09T16:09:35.004042Z"
//  },
//  "sub": "695725d4-a345-4e68-919a-7395cb68484c"
//}
func ValidateToken(logger *logrus.Logger, t string) (*jwt.Token, error) {
	keyForTokenWithLogger := func(token *jwt.Token) (interface{}, error) {
		return keyForToken(logger, token)
	}
	token, err := jwt.Parse(t, keyForTokenWithLogger)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token is invalid")
	}
	// TODO Verify issuer, scope, etc.?
	return token, nil
}

// keyForToken finds a suitable key for validating the
// given token among the public keys provided by Oathkeeper.
func keyForToken(logger *logrus.Logger, token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, errors.New(fmt.Sprintf("unexpected signing method: %v", token.Header["alg"]))
	}
	keySet, err := oathkeeperPublicKeys(logger)
	if err != nil {
		return nil, err
	}
	if reflect.ValueOf(token.Header["kid"]).Kind() != reflect.String {
		return nil, errors.New("invalid jwk header - the kid field is not a string")
	}
	keys := keySet.LookupKeyID(token.Header["kid"].(string))
	if len(keys) == 0 {
		return nil, errors.New("no suitable keys found")
	}
	return keys[0].Materialize()
}

// oathkeeperPublicKeys checks whether we have the
// needed public key cached and if we don't it fetches it and caches it for us.
//
// See https://tools.ietf.org/html/rfc7517
// See https://auth0.com/blog/navigating-rs256-and-jwks/
// See http://self-issued.info/docs/draft-ietf-oauth-json-web-token.html
// Encoding RSA pub key: https://play.golang.org/p/mLpOxS-5Fy
func oathkeeperPublicKeys(logger *logrus.Logger) (*jwk.Set, error) {
	if oathkeeperPubKeys == nil {
		oathkeeperPubKeyURL := "http://" + OathkeeperAddr + "/.well-known/jwks.json"
		logger.Traceln("fetching JWKS from oathkeeper:", oathkeeperPubKeyURL)
		r, err := http.Get(oathkeeperPubKeyURL) // #nosec G107: Potential HTTP request made with variable url
		if err != nil {
			logger.Warningln("ERROR while fetching JWKS from oathkeeper", err)
			return nil, err
		}
		defer func() { _ = r.Body.Close() }()
		var b []byte
		b, err = ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Warningln("ERROR while reading JWKS from oathkeeper", err)
			return nil, err
		}
		var set *jwk.Set
		set, err = jwk.ParseString(string(b))
		if err != nil {
			logger.Warningln("ERROR while parsing JWKS from oathkeeper", err)
			logger.Warningln("JWKS string:", string(b))
			return nil, err
		}
		oathkeeperPubKeys = set
	}
	return oathkeeperPubKeys, nil
}
