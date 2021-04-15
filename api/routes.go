package api

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/NebulousLabs/skynet-accounts/database"
	"github.com/NebulousLabs/skynet-accounts/jwt"

	jwt2 "github.com/dgrijalva/jwt-go"
	"github.com/julienschmidt/httprouter"
	"gitlab.com/NebulousLabs/errors"
)

// buildHTTPRoutes registers all HTTP routes and their handlers.
func (api *API) buildHTTPRoutes() {
	api.staticRouter.POST("/login", api.noValidate(api.loginHandler))
	api.staticRouter.POST("/logout", api.validate(api.logoutHandler))

	api.staticRouter.POST("/track/upload/:skylink", api.validate(api.trackUploadHandler))
	api.staticRouter.POST("/track/download/:skylink", api.validate(api.trackDownloadHandler))
	api.staticRouter.POST("/track/registry/read", api.validate(api.trackRegistryReadHandler))
	api.staticRouter.POST("/track/registry/write", api.validate(api.trackRegistryWriteHandler))

	api.staticRouter.GET("/user", api.validate(api.userHandler))
	api.staticRouter.PUT("/user", api.validate(api.userPutHandler))
	api.staticRouter.GET("/user/limits", api.noValidate(api.userLimitsHandler))
	api.staticRouter.GET("/user/stats", api.validate(api.userStatsHandler))
	api.staticRouter.GET("/user/uploads", api.validate(api.userUploadsHandler))
	api.staticRouter.DELETE("/user/uploads/:uploadId", api.validate(api.userUploadDeleteHandler))
	api.staticRouter.GET("/user/downloads", api.validate(api.userDownloadsHandler))

	api.staticRouter.DELETE("/skylink/:skylink", api.validate(api.skylinkDeleteHandler))

	api.staticRouter.POST("/stripe/webhook", api.noValidate(api.stripeWebhookHandler))
	api.staticRouter.GET("/stripe/prices", api.noValidate(api.stripePricesHandler))
}

// noValidate is a pass-through method used for decorating the request and
// logging relevant data.
func (api *API) noValidate(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		api.logRequest(req)
		h(w, req, ps)
	}
}

// validate ensures that the user making the request has logged in.
func (api *API) validate(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		api.logRequest(req)
		tokenStr, err := tokenFromRequest(req)
		if err != nil {
			api.staticLogger.Traceln("Error fetching token from request:", err)
			api.WriteError(w, err, http.StatusUnauthorized)
			return
		}
		token, err := jwt.ValidateToken(api.staticLogger, tokenStr)
		if err != nil {
			api.staticLogger.Traceln("Error validating token:", err)
			api.WriteError(w, err, http.StatusUnauthorized)
			return
		}
		// Embed the verified token in the context of the request.
		ctx := jwt.ContextWithToken(req.Context(), token)
		h(w, req.WithContext(ctx), ps)
	}
}

// logRequest logs information about the current request.
func (api *API) logRequest(r *http.Request) {
	hasAuth := strings.HasPrefix(r.Header.Get("Authorization"), "Bearer")
	c, err := r.Cookie(CookieName)
	hasCookie := err == nil && c != nil
	api.staticLogger.Tracef("Processing request: %v %v, Auth: %v, Skynet Cookie: %v, Referer: %v, Host: %v, RemoreAddr: %v", r.Method, r.URL, hasAuth, hasCookie, r.Referer(), r.Host, r.RemoteAddr)
}

// tokenFromRequest extracts the JWT token from the request and returns it.
// It first checks the request headers and then the cookies.
func tokenFromRequest(r *http.Request) (string, error) {
	// Check the headers for a token.
	authHeader := r.Header.Get("Authorization")
	parts := strings.Split(authHeader, "Bearer")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1]), nil
	}
	// Check the cookie for a token.
	cookie, err := r.Cookie(CookieName)
	if errors.Contains(err, http.ErrNoCookie) {
		return "", errors.New("no cookie found")
	}
	if err != nil {
		return "", errors.AddContext(err, "cookie exists but it's not valid")
	}
	var value string
	err = secureCookie.Decode(CookieName, cookie.Value, &value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// userFromRequest returns a user object based on the JWT within the request.
// Note that this method does not rely on a token being stored in the context.
func (api *API) userFromRequest(r *http.Request) *database.User {
	t, err := tokenFromRequest(r)
	if err != nil {
		return nil
	}
	token, err := jwt.ValidateToken(api.staticLogger, t)
	if err != nil {
		return nil
	}
	if reflect.ValueOf(token.Claims).Kind() != reflect.ValueOf(jwt2.MapClaims{}).Kind() {
		return nil
	}
	claims := token.Claims.(jwt2.MapClaims)
	if reflect.ValueOf(claims["sub"]).Kind() != reflect.String {
		return nil
	}
	u, err := api.staticDB.UserBySub(r.Context(), claims["sub"].(string), false)
	if err != nil {
		return nil
	}
	return u
}
