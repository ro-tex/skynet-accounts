package jwt

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"time"

	"github.com/SkynetLabs/skynet-accounts/build"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/sirupsen/logrus"
	"gitlab.com/NebulousLabs/errors"
)

var (
	// AccountsJWKS is the public RS key set used by accounts for JWT signing.
	AccountsJWKS jwk.Set = nil

	// AccountsPublicJWKS is a verification-only version of the JWKS.
	// We cannot use the full version of the JWKS for verification.
	AccountsPublicJWKS jwk.Set = nil

	// AccountsJWKSFile defines where to look for the JWKS file.
	AccountsJWKSFile = build.Select(
		build.Var{
			Dev:      "jwks.json",
			Testing:  "fixtures/jwks.json",
			Standard: "/accounts/conf/jwks.json",
		},
	).(string)

	// ErrTokenExpired is returned when a user tries to authenticate with an
	// expired token.
	ErrTokenExpired = errors.New("token expired")

	// PortalName is the issuing service we are using for our JWTs. This
	// value can be overwritten by main.go is PORTAL_DOMAIN is set.
	PortalName = "https://siasky.net"

	// TTL defines the lifetime of the JWT token in seconds.
	TTL = 720 * 3600
)

type (
	// ctxValue is a helper type which makes it safe to register values in the
	// context. If we don't use a custom unexported type it's easy for others
	// to get our value or accidentally overwrite it.
	ctxValue string

	// tokenSession is the bare minimum we need in the `session` claim in our
	// JWTs.
	tokenSession struct {
		Active   bool          `json:"active"`
		Identity tokenIdentity `json:"identity"`
	}
	tokenIdentity struct {
		Traits tokenTraits `json:"traits"`
	}
	tokenTraits struct {
		Email string `json:"email"`
	}
)

// ContextWithToken returns a copy of the given context that contains a token.
func ContextWithToken(ctx context.Context, token jwt.Token) context.Context {
	return context.WithValue(ctx, ctxValue("token"), token)
}

// TokenForUser creates a serialized JWT token for the given user.
//
// The tokens generated by this function are a slimmed down version of the ones
// described in ValidateToken's docstring.
func TokenForUser(email, sub string) (jwt.Token, error) {
	sigAlgo, key, err := signatureAlgoAndKey()
	if err != nil {
		return nil, err
	}
	t, err := tokenForUser(email, sub)
	if err != nil {
		return nil, errors.AddContext(err, "failed to build token")
	}
	bytes, err := jwt.Sign(t, sigAlgo, key)
	if err != nil {
		return nil, errors.New("failed to sign token")
	}
	tk, err := jwt.Parse(bytes)
	if err != nil {
		return nil, errors.New("failed to determine serialize token")
	}
	return tk, nil
}

// TokenFields extracts and returns some fields of interest from the JWT token.
func TokenFields(t jwt.Token) (sub string, email string, token jwt.Token, err error) {
	s, ok := t.Get("sub")
	if !ok || s.(string) == "" {
		err = errors.New("sub field missing")
		return
	}
	sess, ok := t.Get("session")
	if !ok {
		err = errors.New("session field missing")
		return
	}
	session := sess.(map[string]interface{})
	identity := session["identity"].(map[string]interface{})
	traits := identity["traits"].(map[string]interface{})
	if traits != nil {
		email = traits["email"].(string)
	}
	sub = s.(string)
	token = t
	return
}

// TokenFromContext extracts the JWT token from the
// context and returns the contained user sub, claims and the token itself.
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
func TokenFromContext(ctx context.Context) (sub string, email string, token jwt.Token, err error) {
	defer func() {
		// This handles a potential problem with the JWT token that would cause
		// a panic during one of the type cases. It shouldn't happen with a
		// properly formatted token and it's easier to read than constantly
		// checking whether the conversion was successful.
		if e := recover(); e != nil {
			err = errors.New(fmt.Sprintf("failed to parse token from context. error: %v", e))
			return
		}
	}()
	t, ok := ctx.Value(ctxValue("token")).(jwt.Token)
	if !ok {
		err = errors.New(fmt.Sprintf("invalid token type: %s", reflect.TypeOf(ctx.Value(ctxValue("token"))).String()))
		return
	}
	return TokenFields(t)
}

// TokenSerialize is a helper method that allows us to serialize a token.
func TokenSerialize(t jwt.Token) ([]byte, error) {
	sigAlgo, key, err := signatureAlgoAndKey()
	if err != nil {
		return nil, err
	}
	return jwt.Sign(t, sigAlgo, key)
}

// UserDetailsFromJWT extracts the user details from the JWT token embedded in
// the context. We do it that way, so we can call this from anywhere in the code.
func UserDetailsFromJWT(ctx context.Context) (sub, email string, err error) {
	if ctx == nil {
		err = errors.New("Invalid context")
		return
	}
	sub, email, _, err = TokenFromContext(ctx)
	return
}

// ValidateToken verifies the validity of a JWT token, both in terms of validity
// of the signature and expiration time.
//
// Example token:
//
// Header:
//
// {
//  "alg": "RS256",
//  "kid": "a2aa9739-d753-4a0d-87ee-61f101050277",
//  "typ": "JWT"
// }
//
// Payload:
//
// {
//  "exp": 1607594172,
//  "iat": 1607593272,
//  "iss": "https://siasky.net/",
//  "jti": "1e5872ae-71d8-49ec-a550-4fc6163cbbf2",
//  "nbf": 1607593272,
//  "sub": "695725d4-a345-4e68-919a-7395cb68484c"
//  "session": {
//    "active": true,
//    "authenticated_at": "2020-12-09T16:09:35.004003Z",
//    "expires_at": "2020-12-10T16:09:35.004003Z",
//    "issued_at": "2020-12-09T16:09:35.004042Z"
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
//  },
// }
func ValidateToken(t string) (jwt.Token, error) {
	token, err := jwt.Parse([]byte(t), jwt.WithKeySet(AccountsPublicJWKS))
	if err != nil {
		return nil, err
	}
	if token.Expiration().UTC().Before(time.Now().UTC()) {
		return nil, ErrTokenExpired
	}
	return token, nil
}

// LoadAccountsKeySet loads the JSON Web Key Set that we use for signing and
// verifying JWTs and caches it in AccountsJWKS (full version) and
// AccountsPublicJWKS (public key only version).
//
// See https://tools.ietf.org/html/rfc7517
// See https://auth0.com/blog/navigating-rs256-and-jwks/
// See http://self-issued.info/docs/draft-ietf-oauth-json-web-token.html
// Encoding RSA pub key: https://play.golang.org/p/mLpOxS-5Fy
func LoadAccountsKeySet(logger *logrus.Logger) error {
	b, err := ioutil.ReadFile(AccountsJWKSFile)
	if err != nil {
		logger.Warningln("ERROR while reading accounts JWKS", err)
		return err
	}
	set := jwk.NewSet()
	err = json.Unmarshal(b, set)
	if err != nil {
		logger.Warningln("ERROR while parsing accounts JWKS", err)
		logger.Warningln("JWKS string:", string(b))
		return err
	}
	// Cache the key set.
	AccountsJWKS = set
	// Cache a public version of the key set.
	AccountsPublicJWKS, err = jwk.PublicSetOf(AccountsJWKS)
	if err != nil {
		logger.Warningln("ERROR while fetching accounts public JWKS", err)
		AccountsJWKS = nil
		AccountsPublicJWKS = nil
		return err
	}
	return nil
}

// signatureAlgoAndKey is a helper which returns the algorithm and key defined
// by the current JWKS.
func signatureAlgoAndKey() (jwa.SignatureAlgorithm, jwk.Key, error) {
	key, found := AccountsJWKS.Get(0)
	if !found {
		return "", nil, errors.New("JWKS is empty")
	}
	var sigAlgo jwa.SignatureAlgorithm
	for _, sa := range jwa.SignatureAlgorithms() {
		if string(sa) == key.Algorithm() {
			sigAlgo = sa
			break
		}
	}
	if sigAlgo == "" {
		return "", nil, errors.New("failed to determine signature algorithm")
	}
	return sigAlgo, key, nil
}

// tokenForUser is a helper method that puts together an unsigned token based
// on the provided values.
func tokenForUser(emailAddr, sub string) (jwt.Token, error) {
	if emailAddr == "" || sub == "" {
		return nil, errors.New("email and sub cannot be empty")
	}
	session := tokenSession{
		Active: true,
		Identity: tokenIdentity{
			Traits: tokenTraits{
				Email: emailAddr,
			},
		},
	}
	now := time.Now().UTC()
	t := jwt.New()
	err1 := t.Set("exp", now.Unix()+int64(TTL))
	err2 := t.Set("iat", now.Unix())
	err3 := t.Set("iss", PortalName)
	err4 := t.Set("sub", sub)
	err5 := t.Set("session", session)
	err := errors.Compose(err1, err2, err3, err4, err5)
	if err != nil {
		return nil, err
	}
	return t, nil
}
